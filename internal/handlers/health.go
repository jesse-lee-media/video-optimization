package handlers

import (
	"net/http"

	"video-optimization/internal/logger"
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK\n")); err != nil {
		logger.Logger.Errorw("Failed to write response", "error", err)
	}
}
