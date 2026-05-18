package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	userports "github.com/MathewM27/busTrack-alebus/application/user/ports"
)

func TestExpoSender_RetriesOn429(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := atomic.AddInt32(&calls, 1)
		if c == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"status": "ok", "id": "t1"}},
		})
	}))
	defer srv.Close()

	sender := NewExpoSender(ExpoSenderConfig{
		PushURL:    srv.URL,
		Timeout:    2 * time.Second,
		MaxRetries: 1,
		MinBackoff: 1 * time.Millisecond,
		MaxBackoff: 1 * time.Millisecond,
	})

	rep, err := sender.SendWithReport(context.Background(), []userports.PushMessage{{To: "ExponentPushToken[abc]", Title: "t", Body: "b"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rep.InvalidTokens) != 0 {
		t.Fatalf("expected 0 invalid tokens, got %v", rep.InvalidTokens)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", got)
	}
}

func TestExpoSender_ReportsInvalidTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"status":  "error",
				"message": "DeviceNotRegistered",
				"details": map[string]any{"error": "DeviceNotRegistered"},
			}},
		})
	}))
	defer srv.Close()

	sender := NewExpoSender(ExpoSenderConfig{PushURL: srv.URL, Timeout: 2 * time.Second})
	rep, err := sender.SendWithReport(context.Background(), []userports.PushMessage{{To: "ExponentPushToken[abc]", Title: "t", Body: "b"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(rep.InvalidTokens); got != 1 {
		t.Fatalf("expected 1 invalid token, got %d", got)
	}
	if rep.InvalidTokens[0] != "ExponentPushToken[abc]" {
		t.Fatalf("unexpected invalid token: %q", rep.InvalidTokens[0])
	}
}
