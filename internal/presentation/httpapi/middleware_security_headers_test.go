package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersMiddleware_HTTP_NoHSTS(t *testing.T) {
	h := TrustedProxyMiddleware(nil)(SecurityHeadersMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("Strict-Transport-Security") != "" {
		t.Fatalf("expected no HSTS")
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected nosniff")
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected DENY")
	}
}

func TestSecurityHeadersMiddleware_HTTPS_SetsHSTS(t *testing.T) {
	h := TrustedProxyMiddleware([]string{"10.0.0.5"})(SecurityHeadersMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	got := rr.Header().Get("Strict-Transport-Security")
	if got == "" {
		t.Fatalf("expected HSTS")
	}
}
