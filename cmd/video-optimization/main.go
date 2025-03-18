package main

import (
	"net/http"

	"video-optimization/internal/environment"
	"video-optimization/internal/handlers"
	"video-optimization/internal/logger"
	"video-optimization/internal/middleware"
	"video-optimization/internal/s3"
)

func main() {
	logger.Init()
	environment.Init()
	s3.Init()

	http.Handle("/thumbnail", middleware.Auth(http.HandlerFunc(handlers.Thumbnail)))
	http.Handle("/delete", middleware.Auth(http.HandlerFunc(handlers.Delete)))
	http.HandleFunc("/health", http.HandlerFunc(handlers.Health))

	handler := middleware.CORS(middleware.RateLimit(http.DefaultServeMux))

	port := "8080"
	logger.Logger.Infow("Starting server", "port", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		logger.Logger.Fatalf("Server failed to start: %v", err)
	}
}
