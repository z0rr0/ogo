// Package middleware provides middleware functions for HTTP handlers.
package middleware

import (
	"log"
	"math/rand"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and calls the underlying WriteHeader method.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the status code and calls the underlying Write method.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// Logging wraps the handler and logs requests using the provided logger
func Logging(h http.Handler, loggerDebug, loggerInfo *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// pseudo random number
		requestID := rand.Uint64()
		urlPath := r.URL.Path
		method := r.Method
		start := time.Now()

		loggerDebug.Printf("request [%d]: %s %s from %s", requestID, method, urlPath, r.RemoteAddr)

		wrapped := &responseWriter{ResponseWriter: w}
		h.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		loggerDebug.Printf("response [%d]: %s duration: %s", requestID, urlPath, duration.String())
		loggerInfo.Printf("response [%d]: %s %s: %d", requestID, method, urlPath, wrapped.statusCode)
	})
}
