package middleware

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"video-optimization/internal/environment"
	"video-optimization/internal/logger"

	"golang.org/x/time/rate"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env := environment.GetEnvironment()

		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "API-Key ") {
			apiKey := strings.TrimPrefix(authHeader, "API-Key ")
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(env.VideoOptimizationApiKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			logger.Logger.Warnw("Unauthorized access attempt with invalid API key", "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		logger.Logger.Warnw("Unauthorized access attempt missing API key", "remote_addr", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func CORS(next http.Handler) http.HandlerFunc {
	env := environment.GetEnvironment()
	allowedOrigins := []string{env.ServerUrl}

	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if slices.Contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				logger.Logger.Warnw("Origin not allowed", "origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

var (
	visitors = make(map[string]*rate.Limiter)
	mu       sync.Mutex
)

func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(time.Second), 5)
		visitors[ip] = limiter
		logger.Logger.Infow("Created new rate limiter", "ip", ip)
	}
	return limiter
}

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}

		limiter := getVisitor(ip)
		if !limiter.Allow() {
			logger.Logger.Warnw("Rate limit exceeded", "ip", ip)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
