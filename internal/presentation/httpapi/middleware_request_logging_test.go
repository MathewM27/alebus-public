package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLoggingMiddleware_EmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	oldOut := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
	}()

	h := RequestLoggingMiddleware(DefaultRequestLoggingConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hi"))
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/health?x=1", nil)
	// Inject request_id so log line includes it.
	req = req.WithContext(context.WithValue(req.Context(), RequestIDContextKey, "rid-1"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	line := bytes.TrimSpace(buf.Bytes())
	if len(line) == 0 {
		t.Fatalf("expected log line")
	}

	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("expected JSON log line, got %q: %v", string(line), err)
	}
	if m["type"] != "request" {
		t.Fatalf("expected type=request, got %v", m["type"])
	}
	if m["request_id"] != "rid-1" {
		t.Fatalf("expected request_id=rid-1, got %v", m["request_id"])
	}
	if m["status"] != float64(http.StatusTeapot) {
		t.Fatalf("expected status=418, got %v", m["status"])
	}
	if m["path"] != "/api/v1/health" {
		t.Fatalf("expected path without query, got %v", m["path"])
	}
}
