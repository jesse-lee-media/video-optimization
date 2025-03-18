package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"video-optimization/internal/logger"
	"video-optimization/internal/s3"
)

type ThumbnailRequest struct {
	Filename string `json:"filename"`
}

type ThumbnailResponse struct {
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
	Height   int    `json:"height"`
	Width    int    `json:"width"`
	MimeType string `json:"mimeType"`
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

func Thumbnail(w http.ResponseWriter, r *http.Request) {
	var req ThumbnailRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Logger.Errorw("Failed to decode Thumbnail request", "error", err)
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.Filename == "" {
		logger.Logger.Warn("Filename is empty in Thumbnail request")
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	baseFilename := filepath.Base(req.Filename)
	filename := getFilename(baseFilename)
	inputPath := filepath.Join(os.TempDir(), baseFilename)
	thumbnailFileName := filename + "_thumbnail.png"
	thumbnailPath := filepath.Join(os.TempDir(), thumbnailFileName)

	logger.Logger.Infow("Downloading file from S3", "filename", req.Filename)
	if err := s3.Download(ctx, baseFilename, inputPath); err != nil {
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
	logger.Logger.Infow("Successfully generated thumbnail", "thumbnailFileName", thumbnailFileName)
	defer func() {
		if err := os.Remove(thumbnailPath); err != nil {
			logger.Logger.Errorw("Error removing thumbnail file", "path", thumbnailPath, "error", err)
		}
	}()

	logger.Logger.Infow("Uploading thumbnail to S3", "thumbnailFileName", thumbnailFileName)
	if _, err := s3.Upload(ctx, thumbnailPath, thumbnailFileName, "image/png"); err != nil {
		logger.Logger.Errorw("Failed to upload thumbnail to S3", "thumbnailFileName", thumbnailFileName, "error", err)
		http.Error(w, fmt.Sprintf("failed to upload thumbnail: %v", err), http.StatusInternalServerError)
		return
	}
	logger.Logger.Infow("Successfully uploaded thumbnail", "thumbnailFileName", thumbnailFileName)

	thumbInfo, err := os.Stat(thumbnailPath)
	if err != nil {
		logger.Logger.Errorw("Failed to get thumbnail file info", "thumbnailPath", thumbnailPath, "error", err)
		http.Error(w, fmt.Sprintf("failed to get thumbnail file info: %v", err), http.StatusInternalServerError)
		return
	}
	thumbFilesize := thumbInfo.Size()

	thumbWidth, thumbHeight, err := getImageDimensions(thumbnailPath)
	if err != nil {
		logger.Logger.Warnw("Warning: failed to get thumbnail dimensions", "error", err)
		thumbWidth, thumbHeight = 0, 0
	}

	resp := ThumbnailResponse{
		Filename: thumbnailFileName,
		Filesize: thumbFilesize,
		Height:   thumbHeight,
		Width:    thumbWidth,
		MimeType: "image/png",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Logger.Errorw("Failed to encode Thumbnail response", "error", err)
	}
}
