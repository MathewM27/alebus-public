package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// TestRequestIDMiddlewareGeneratesID verifies that a request ID is generated if not provided.
func TestRequestIDMiddlewareGeneratesID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response header contains X-Request-ID
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Errorf("expected X-Request-ID in response header, got empty")
	}

	// Verify it's a valid UUID
	if _, err := uuid.Parse(requestID); err != nil {
		t.Errorf("expected valid UUID, got: %s", requestID)
	}
}

// TestRequestIDMiddlewareUsesClientID verifies that client-provided X-Request-ID is used.
func TestRequestIDMiddlewareUsesClientID(t *testing.T) {
	clientID := "client-request-123"

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Request-ID", clientID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response header contains the client-provided ID
	responseID := w.Header().Get("X-Request-ID")
	if responseID != clientID {
		t.Errorf("expected X-Request-ID=%s, got %s", clientID, responseID)
	}
}

// TestRequestIDMiddlewareInjectsContext verifies request ID is available in context.
func TestRequestIDMiddlewareInjectsContext(t *testing.T) {
	clientID := "test-id-456"
	var capturedID string

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Request-ID", clientID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if capturedID != clientID {
		t.Errorf("expected context request ID=%s, got %s", clientID, capturedID)
	}
}

// TestGetRequestIDFromContext_MissingID verifies empty string when not found.
func TestGetRequestIDFromContext_MissingID(t *testing.T) {
	ctx := context.Background()
	id := GetRequestIDFromContext(ctx)
	if id != "" {
		t.Errorf("expected empty string for missing request ID, got %s", id)
	}
}

// TestRequestIDMiddlewareWithDifferentMethods verifies it works across HTTP methods.
func TestRequestIDMiddlewareWithDifferentMethods(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		req := httptest.NewRequest(method, "http://example.com/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Errorf("expected X-Request-ID for %s, got empty", method)
		}
	}
}
