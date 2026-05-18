package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_SetsContentTypeAndStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSON(rr, http.StatusCreated, map[string]any{"x": 1})

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", ct)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if decoded["x"] != float64(1) {
		t.Fatalf("expected x=1, got %v", decoded["x"])
	}
}

func TestWriteError_EnvelopeShape(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteError(rr, http.StatusBadRequest, "invalid_request", "missing param", map[string]any{"param": "originLat"})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var decoded ErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if decoded.Error.Code != "invalid_request" {
		t.Fatalf("expected code invalid_request, got %q", decoded.Error.Code)
	}
	if decoded.Error.Message != "missing param" {
		t.Fatalf("expected message missing param, got %q", decoded.Error.Message)
	}
	if decoded.Error.Details["param"] != "originLat" {
		t.Fatalf("expected details.param originLat, got %v", decoded.Error.Details["param"])
	}
}
