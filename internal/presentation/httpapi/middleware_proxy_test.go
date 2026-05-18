package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedProxyMiddleware_TrustedProxy_UsesXForwardedHeaders(t *testing.T) {
	h := TrustedProxyMiddleware([]string{"10.0.0.5"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := ClientIPFromContext(r.Context()); got != "203.0.113.50" {
			t.Fatalf("client ip = %q", got)
		}
		if got := ClientProtoFromContext(r.Context()); got != "https" {
			t.Fatalf("client proto = %q", got)
		}
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	req.Header.Set("X-Forwarded-Proto", "https")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTrustedProxyMiddleware_UntrustedProxy_IgnoresXForwardedHeaders(t *testing.T) {
	h := TrustedProxyMiddleware([]string{"10.0.0.5"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := ClientIPFromContext(r.Context()); got != "192.0.2.10" {
			t.Fatalf("client ip = %q", got)
		}
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "192.0.2.10:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
}
