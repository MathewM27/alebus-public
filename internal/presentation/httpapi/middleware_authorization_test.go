package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireScope(t *testing.T) {
	next := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
	h := RequireScope(ScopeOps)(next)

	// Missing scope => forbidden
	{
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example/", nil)
		h(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	}

	// Public => forbidden
	{
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example/", nil)
		req = req.WithContext(withScope(req.Context(), ScopePublic))
		h(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	}

	// Ops => ok
	{
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example/", nil)
		req = req.WithContext(withScope(req.Context(), ScopeOps))
		h(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	}

	// Admin => ok
	{
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example/", nil)
		req = req.WithContext(withScope(req.Context(), ScopeAdmin))
		h(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	}
}
