package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuditLoggingMiddleware_LogsOnlyMutations(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	prevFlags := log.Flags()
	prevPrefix := log.Prefix()
	log.SetOutput(&buf)
	log.SetFlags(0)
	log.SetPrefix("")
	defer func() {
		log.SetOutput(prev)
		log.SetFlags(prevFlags)
		log.SetPrefix(prevPrefix)
	}()

	h := AuditLoggingMiddleware(DefaultAuditLoggingConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// GET should not emit audit logs.
	buf.Reset()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/routes", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if buf.Len() != 0 {
		t.Fatalf("expected no audit log for GET, got %q", buf.String())
	}

	// POST should emit audit logs.
	buf.Reset()
	req = httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/routes?x=1", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("expected JSON log line, got err=%v line=%q", err, buf.String())
	}
	if got["type"] != "audit" {
		t.Fatalf("expected type=audit, got %v", got["type"])
	}
	if got["action"] == "" {
		t.Fatalf("expected action to be set")
	}
	if got["status"] != float64(http.StatusNoContent) {
		t.Fatalf("expected status %d, got %v", http.StatusNoContent, got["status"])
	}
	if got["path"] != "/api/v1/routes" {
		t.Fatalf("expected path without query, got %v", got["path"])
	}
}
