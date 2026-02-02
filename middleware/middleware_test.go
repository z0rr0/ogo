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

func TestGetRequestID(t *testing.T) {
	id, err := getRequestID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be hex encoded, so 8 bytes = 16 hex chars
	expectedLen := requestIDSize * 2
	if len(id) != expectedLen {
		t.Errorf("expected request ID length %d, got %d", expectedLen, len(id))
	}

	// Should be valid hex
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("request ID contains invalid hex character: %c", c)
		}
	}
}

func TestGetRequestID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	const iterations = 1000

	for i := range iterations {
		id, err := getRequestID()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Errorf("duplicate request ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestLogging_ContainsRequestID(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	slog.SetDefault(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Logging(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "id=") {
		t.Error("log should contain request ID field")
	}
}

func TestLogging_RequestIDConsistentAcrossLogLines(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	slog.SetDefault(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Logging(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	lines := strings.Split(strings.TrimSpace(logOutput), "\n")

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d", len(lines))
	}

	// Extract request IDs from each line
	var ids []string
	for _, line := range lines {
		// Find id= in the line
		_, after, found := strings.Cut(line, "id=")
		if !found {
			t.Errorf("log line missing id field: %s", line)
			continue
		}
		// Extract the ID value (until next space)
		id, _, _ := strings.Cut(after, " ")
		ids = append(ids, id)
	}

	// All IDs should be the same
	if len(ids) > 1 {
		for i := 1; i < len(ids); i++ {
			if ids[i] != ids[0] {
				t.Errorf("request ID mismatch: line 0 has %s, line %d has %s", ids[0], i, ids[i])
			}
		}
	}
}

func TestLogging_DifferentRequestsHaveDifferentIDs(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	slog.SetDefault(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Logging(handler)

	// Make first request
	req1 := httptest.NewRequest(http.MethodGet, "/first", nil)
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	firstOutput := logBuf.String()
	logBuf.Reset()

	// Make second request
	req2 := httptest.NewRequest(http.MethodGet, "/second", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	secondOutput := logBuf.String()

	// Extract IDs
	extractID := func(output string) string {
		_, after, found := strings.Cut(output, "id=")
		if !found {
			return ""
		}
		id, _, _ := strings.Cut(after, " ")
		return id
	}

	id1 := extractID(firstOutput)
	id2 := extractID(secondOutput)

	if id1 == "" || id2 == "" {
		t.Fatal("failed to extract request IDs from log output")
	}

	if id1 == id2 {
		t.Errorf("different requests should have different IDs, both got: %s", id1)
	}
}
