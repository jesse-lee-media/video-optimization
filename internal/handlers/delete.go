package handlers

import (
	"encoding/json"
	"net/http"

	"video-optimization/internal/logger"
	"video-optimization/internal/s3"
)

type DeleteRequest struct {
	Filenames []string `json:"filenames"`
}

type DeleteResponse struct {
	Message string `json:"message"`
}

func Delete(w http.ResponseWriter, r *http.Request) {
	var req DeleteRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Logger.Errorw("Failed to decode delete request", "error", err)
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	if len(req.Filenames) == 0 {
		logger.Logger.Warn("No filenames provided for deletion")
		http.Error(w, "at least one filename is required", http.StatusBadRequest)
		return
	}

	for _, filename := range req.Filenames {
		logger.Logger.Infow("Deleting file from S3", "filename", filename)
		if err := s3.Delete(r.Context(), filename); err != nil {
			logger.Logger.Errorw("Failed to delete file from S3", "filename", filename, "error", err)
			http.Error(w, "failed to delete object", http.StatusInternalServerError)
			return
		}
		logger.Logger.Infow("Successfully deleted file from S3", "filename", filename)
	}

	resp := DeleteResponse{
		Message: "Files deleted",
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Logger.Errorw("Failed to encode delete response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
