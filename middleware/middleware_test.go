package middleware

import (
	"bufio"
	"bytes"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("expected status code %d, got %d", http.StatusNotFound, rw.statusCode)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected underlying status code %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestResponseWriter_Write_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	body := []byte("hello")
	n, err := rw.Write(body)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(body) {
		t.Errorf("expected %d bytes written, got %d", len(body), n)
	}
	if rw.statusCode != http.StatusOK {
		t.Errorf("expected default status code %d, got %d", http.StatusOK, rw.statusCode)
	}
}

func TestResponseWriter_Write_ExplicitStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	rw.WriteHeader(http.StatusCreated)
	_, err := rw.Write([]byte("created"))

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if rw.statusCode != http.StatusCreated {
		t.Errorf("expected status code %d, got %d", http.StatusCreated, rw.statusCode)
	}
}

func TestResponseWriter_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	// Should not panic even if underlying doesn't support Flush
	rw.Flush()

	// httptest.ResponseRecorder implements Flusher, verify it was called
	if !rec.Flushed {
		t.Error("expected Flush to be called on underlying ResponseWriter")
	}
}

// mockResponseWriter is a minimal ResponseWriter that doesn't implement Flusher or Hijacker
type mockResponseWriter struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return m.body.Write(b)
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.statusCode = code
}

func TestResponseWriter_Flush_NotSupported(t *testing.T) {
	mock := &mockResponseWriter{}
	rw := &responseWriter{ResponseWriter: mock}

	// Should not panic when underlying doesn't support Flush
	rw.Flush()
}

func TestResponseWriter_Hijack_NotSupported(t *testing.T) {
	mock := &mockResponseWriter{}
	rw := &responseWriter{ResponseWriter: mock}

	conn, buf, err := rw.Hijack()

	if conn != nil {
		t.Error("expected nil conn")
	}
	if buf != nil {
		t.Error("expected nil buffer")
	}
	if err == nil {
		t.Fatal("expected error when Hijacker not implemented")
	}
	if !strings.Contains(err.Error(), "http.Hijacker not implemented") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// mockHijacker implements http.Hijacker for testing
type mockHijacker struct {
	mockResponseWriter
	hijacked bool
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true
	return nil, nil, nil
}

func TestResponseWriter_Hijack_Supported(t *testing.T) {
	mock := &mockHijacker{}
	rw := &responseWriter{ResponseWriter: mock}

	_, _, err := rw.Hijack()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mock.hijacked {
		t.Error("expected Hijack to be called on underlying ResponseWriter")
	}
}

func TestLogging(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	slog.SetDefault(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	})

	wrapped := Logging(handler)

	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Check log output contains expected info
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "GET") {
		t.Error("log should contain method")
	}
	if !strings.Contains(logOutput, "/test/path") {
		t.Error("log should contain URL path")
	}
	if !strings.Contains(logOutput, "127.0.0.1:12345") {
		t.Error("log should contain remote address")
	}
	if !strings.Contains(logOutput, "duration") {
		t.Error("log should contain duration")
	}
	if !strings.Contains(logOutput, "200") {
		t.Error("log should contain status code")
	}
}

func TestLogging_ErrorStatus(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	slog.SetDefault(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	wrapped := Logging(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/error", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "500") {
		t.Error("log should contain 500 status code")
	}
	if !strings.Contains(logOutput, "POST") {
		t.Error("log should contain POST method")
	}
}

func TestLogging_ImplicitOKStatus(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	slog.SetDefault(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write without explicit WriteHeader - should default to 200
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	})

	wrapped := Logging(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "200") {
		t.Error("log should contain 200 status code for implicit OK")
	}
}
