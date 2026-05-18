package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRecoveryMiddlewareCatchesPanic verifies that the middleware recovers from panics.
func TestRecoveryMiddlewareCatchesPanic(t *testing.T) {
	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with recovery middleware
	handler := RecoveryMiddleware(panicHandler)

	// Make request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	// Verify JSON error envelope
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	// Parse response body
	body := w.Body.String()
	var errEnv struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &errEnv); err != nil {
		t.Fatalf("expected valid JSON error, got: %s, error: %v", body, err)
	}

	if errEnv.Error.Code != "internal_error" {
		t.Errorf("expected code 'internal_error', got %s", errEnv.Error.Code)
	}
}

// TestRecoveryMiddlewarePassesThroughNormally verifies no side effects on normal requests.
func TestRecoveryMiddlewarePassesThroughNormally(t *testing.T) {
	// Normal handler
	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Wrap with recovery middleware
	handler := RecoveryMiddleware(normalHandler)

	// Make request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify body is unmodified
	body := w.Body.String()
	var result map[string]string
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s, error: %v", body, err)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status='ok', got %s", result["status"])
	}
}

// TestRecoveryMiddlewarePreservesRequestInfo verifies request context is available in panic log.
func TestRecoveryMiddlewarePreservesRequestInfo(t *testing.T) {
	// Capture log output
	var logBuffer bytes.Buffer
	oldStdout := log.Writer()
	log.SetOutput(&logBuffer)
	defer log.SetOutput(oldStdout)

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic in TestRecoveryMiddlewarePreservesRequestInfo")
	})

	handler := RecoveryMiddleware(panicHandler)

	req := httptest.NewRequest("POST", "http://example.com/api/v1/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify panic was logged with method and URI
	logOutput := logBuffer.String()
	if !bytes.Contains([]byte(logOutput), []byte("POST")) {
		t.Errorf("expected POST in log, got: %s", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("/api/v1/test")) {
		t.Errorf("expected /api/v1/test in log, got: %s", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("PANIC")) {
		t.Errorf("expected PANIC in log, got: %s", logOutput)
	}
}
