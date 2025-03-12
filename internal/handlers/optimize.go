package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"video-optimization/internal/logger"
	"video-optimization/internal/s3"

	"github.com/google/uuid"
)

type OptimizeRequest struct {
	Filename string            `json:"filename"`
	Options  map[string]string `json:"options"`
}

type OptimizedVideo struct {
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
	Height   int    `json:"height"`
	Width    int    `json:"width"`
	MimeType string `json:"mimeType"`
}

type Thumbnail struct {
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
	Height   int    `json:"height"`
	Width    int    `json:"width"`
	MimeType string `json:"mimeType"`
}

type OptimizeResponse struct {
	OptimizedVideo OptimizedVideo `json:"optimizedVideo"`
	Thumbnail      Thumbnail      `json:"thumbnail"`
}

func resolutionToScale(res string) (string, bool) {
	if strings.HasSuffix(res, "p") {
		numStr := strings.TrimSuffix(res, "p")
		if _, err := strconv.Atoi(numStr); err == nil {
			return fmt.Sprintf("-1:%s", numStr), true
		}
	}
	return "", false
}

func runCommand(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*30)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Logger.Errorw("Error running command",
			"command", name,
			"args", args,
			"error", err,
			"output", string(output))
		return fmt.Errorf("command %s failed: %w", name, err)
	}
	return nil
}

func getFilename(filename string) string {
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)]
}

func getVideoDimensions(filePath string) (int, int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get video dimensions: %w", err)
	}
	dims := strings.TrimSpace(string(output))
	parts := strings.Split(dims, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected dimensions format: %s", dims)
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse width: %w", err)
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse height: %w", err)
	}
	return width, height, nil
}

func getImageDimensions(filePath string) (int, int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open image file: %w", err)
	}
	defer f.Close()

	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode image config: %w", err)
	}
	return config.Width, config.Height, nil
}

func Optimize(w http.ResponseWriter, r *http.Request) {
	var req OptimizeRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Logger.Errorw("Failed to decode Optimize request", "error", err)
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.Filename == "" {
		logger.Logger.Warn("Filename is empty in Optimize request")
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	req.Filename = filepath.Base(req.Filename)

	ctx := r.Context()
	inputPath := filepath.Join(os.TempDir(), req.Filename)
	format := "webm"
	if f, ok := req.Options["format"]; ok && f != "" {
		format = f
	}

	filename := getFilename(req.Filename)
	suffix := uuid.NewString()
	optimizedFileName := filename + "_" + suffix + "." + format
	optimizedPath := filepath.Join(os.TempDir(), optimizedFileName)
	thumbnailFileName := filename + "_" + suffix + "_thumbnail.png"
	thumbnailPath := filepath.Join(os.TempDir(), thumbnailFileName)

	logger.Logger.Infow("Downloading file from S3", "filename", req.Filename)
	if err := s3.Download(ctx, req.Filename, inputPath); err != nil {
		logger.Logger.Errorw("Failed to download file from S3", "filename", req.Filename, "error", err)
		http.Error(w, fmt.Sprintf("failed to download \"%s\" from S3: %v", req.Filename, err), http.StatusInternalServerError)
		return
	}
	logger.Logger.Infow("Successfully downloaded file from S3", "filename", req.Filename)
	defer func() {
		if err := os.Remove(inputPath); err != nil {
			logger.Logger.Errorw("Error removing input file", "path", inputPath, "error", err)
		}
	}()

	args := []string{"-i", inputPath}
	if res, ok := req.Options["resolution"]; ok && res != "" {
		if scale, found := resolutionToScale(res); found {
			args = append(args, "-vf", fmt.Sprintf("scale=%s", scale))
		}
	}
	args = append(args,
		"-c:v", "libvpx-vp9",
		"-crf", "30",
		"-b:v", "1M",
		"-c:a", "libopus",
		"-b:a", "128k",
		"-y",
		optimizedPath,
	)

	logger.Logger.Infow("Optimizing video", "inputPath", inputPath, "optimizedPath", optimizedPath, "args", args)
	if err := runCommand(ctx, "ffmpeg", args...); err != nil {
		logger.Logger.Errorw("Video optimization failed", "error", err)
		http.Error(w, fmt.Sprintf("video optimization failed: %v", err), http.StatusInternalServerError)
		return
	}
	logger.Logger.Infow("Successfully optimized video", "optimizedFileName", optimizedFileName)
	defer func() {
		if err := os.Remove(optimizedPath); err != nil {
			logger.Logger.Errorw("Error removing optimized file", "path", optimizedPath, "error", err)
		}
	}()

	thumbArgs := []string{
		"-i", inputPath,
		"-ss", "00:00:01",
		"-vframes", "1",
		"-y",
		thumbnailPath,
	}
	logger.Logger.Infow("Generating thumbnail", "thumbnailPath", thumbnailPath)
	if err := runCommand(ctx, "ffmpeg", thumbArgs...); err != nil {
		logger.Logger.Errorw("Thumbnail generation failed", "error", err)
		http.Error(w, fmt.Sprintf("thumbnail generation failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := os.Remove(thumbnailPath); err != nil {
			logger.Logger.Errorw("Error removing thumbnail file", "path", thumbnailPath, "error", err)
		}
	}()

	convertedThumbnailFileName := filename + "_" + suffix + "_thumbnail_converted.png"
	convertedThumbnailPath := filepath.Join(os.TempDir(), convertedThumbnailFileName)
	convertArgs := []string{
		"-i", thumbnailPath,
		"-y",
		convertedThumbnailPath,
	}
	logger.Logger.Infow("Converting thumbnail", "convertedThumbnailPath", convertedThumbnailPath)
	if err := runCommand(ctx, "ffmpeg", convertArgs...); err != nil {
		logger.Logger.Errorw("Thumbnail conversion failed", "error", err)
		http.Error(w, fmt.Sprintf("thumbnail conversion failed: %v", err), http.StatusInternalServerError)
		return
	}
	logger.Logger.Infow("Successfully generated and converted thumbnail", "convertedThumbnailFileName", convertedThumbnailFileName)
	defer func() {
		if err := os.Remove(convertedThumbnailPath); err != nil {
			logger.Logger.Errorw("Error removing converted thumbnail file", "path", convertedThumbnailPath, "error", err)
		}
	}()

	logger.Logger.Infow("Uploading optimized video to S3", "optimizedFileName", optimizedFileName)
	if _, err := s3.Upload(ctx, optimizedPath, optimizedFileName, "video/"+format); err != nil {
		logger.Logger.Errorw("Failed to upload video to S3", "optimizedFileName", optimizedFileName, "error", err)
		http.Error(w, fmt.Sprintf("failed to upload video: %v", err), http.StatusInternalServerError)
		return
	}
	logger.Logger.Infow("Successfully uploaded optimized video", "optimizedFileName", optimizedFileName)

	logger.Logger.Infow("Uploading thumbnail to S3", "convertedThumbnailFileName", convertedThumbnailFileName)
	if _, err := s3.Upload(ctx, convertedThumbnailPath, convertedThumbnailFileName, "image/png"); err != nil {
		logger.Logger.Errorw("Failed to upload thumbnail to S3", "convertedThumbnailFileName", convertedThumbnailFileName, "error", err)
		http.Error(w, fmt.Sprintf("failed to upload thumbnail: %v", err), http.StatusInternalServerError)
		return
	}
	logger.Logger.Infow("Successfully uploaded thumbnail", "convertedThumbnailFileName", convertedThumbnailFileName)

	videoInfo, err := os.Stat(optimizedPath)
	if err != nil {
		logger.Logger.Errorw("Failed to get video file info", "optimizedPath", optimizedPath, "error", err)
		http.Error(w, fmt.Sprintf("failed to get video file info: %v", err), http.StatusInternalServerError)
		return
	}
	videoFilesize := videoInfo.Size()

	videoWidth, videoHeight, err := getVideoDimensions(optimizedPath)
	if err != nil {
		logger.Logger.Warnw("Warning: failed to get video dimensions", "error", err)
		videoWidth, videoHeight = 0, 0
	}

	thumbInfo, err := os.Stat(convertedThumbnailPath)
	if err != nil {
		logger.Logger.Errorw("Failed to get thumbnail file info", "convertedThumbnailPath", convertedThumbnailPath, "error", err)
		http.Error(w, fmt.Sprintf("failed to get thumbnail file info: %v", err), http.StatusInternalServerError)
		return
	}
	thumbFilesize := thumbInfo.Size()

	thumbWidth, thumbHeight, err := getImageDimensions(convertedThumbnailPath)
	if err != nil {
		logger.Logger.Warnw("Warning: failed to get thumbnail dimensions", "error", err)
		thumbWidth, thumbHeight = 0, 0
	}

	resp := OptimizeResponse{
		OptimizedVideo: OptimizedVideo{
			Filename: optimizedFileName,
			Filesize: videoFilesize,
			Height:   videoHeight,
			Width:    videoWidth,
			MimeType: "video/" + format,
		},
		Thumbnail: Thumbnail{
			Filename: convertedThumbnailFileName,
			Filesize: thumbFilesize,
			Height:   thumbHeight,
			Width:    thumbWidth,
			MimeType: "image/png",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Logger.Errorw("Failed to encode Optimize response", "error", err)
	}
}
