// Package middleware provides middleware functions for HTTP handlers.
package middleware

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	// requestIDSize is the size of the request ID in bytes.
	requestIDSize = 8
)

var (
	requestIDPool = sync.Pool{
		New: func() any {
			return make([]byte, requestIDSize)
		},
	}
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

// Flush implements http.Flusher interface if the underlying ResponseWriter supports it.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker interface if the underlying ResponseWriter supports it.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("http.Hijacker not implemented by underlying ResponseWriter")
}

// Logging wraps the handler and logs requests using the provided logger
func Logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, err := getRequestID()
		if err != nil {
			slog.Error("requestID", "error", err)
			requestID = strconv.Itoa(time.Now().Nanosecond()) // fallback
		}
		start := time.Now()

		logger := slog.Default().With("id", requestID, "method", r.Method, "path", r.URL.Path)
		logger.Info("request", "remote", r.RemoteAddr)

		wrapped := &responseWriter{ResponseWriter: w}
		h.ServeHTTP(wrapped, r)

		duration := time.Since(start).Round(time.Millisecond)
		logger.Info("response", "duration", duration, "status", wrapped.statusCode)
	})
}

// getRequestID generates a random request ID using a pool of bytes.
func getRequestID() (string, error) {
	b := requestIDPool.Get().([]byte)
	defer requestIDPool.Put(b)

	n, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	if n != requestIDSize {
		return "", errors.New("unexpected number of bytes read")
	}

	return hex.EncodeToString(b), nil
}
