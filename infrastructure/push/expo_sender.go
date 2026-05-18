package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	userports "github.com/MathewM27/busTrack-alebus/application/user/ports"
)

const expoPushURL = "https://exp.host/--/api/v2/push/send"

type ExpoSender struct {
	httpClient  *http.Client
	accessToken string
	pushURL     string
	maxRetries  int
	minBackoff  time.Duration
	maxBackoff  time.Duration
}

type ExpoSenderConfig struct {
	AccessToken string
	Timeout     time.Duration
	PushURL     string

	// Retry settings for transient failures.
	// Retries apply to HTTP 429 and 5xx responses.
	MaxRetries int
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

func NewExpoSender(cfg ExpoSenderConfig) *ExpoSender {
	t := cfg.Timeout
	if t <= 0 {
		t = 8 * time.Second
	}
	url := strings.TrimSpace(cfg.PushURL)
	if url == "" {
		url = expoPushURL
	}

	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	minBackoff := cfg.MinBackoff
	if minBackoff <= 0 {
		minBackoff = 200 * time.Millisecond
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 2 * time.Second
	}
	if maxBackoff < minBackoff {
		maxBackoff = minBackoff
	}

	return &ExpoSender{
		httpClient:  &http.Client{Timeout: t},
		accessToken: cfg.AccessToken,
		pushURL:     url,
		maxRetries:  maxRetries,
		minBackoff:  minBackoff,
		maxBackoff:  maxBackoff,
	}
}

func (s *ExpoSender) Send(ctx context.Context, messages []userports.PushMessage) error {
	_, err := s.SendWithReport(ctx, messages)
	return err
}

func (s *ExpoSender) SendWithReport(ctx context.Context, messages []userports.PushMessage) (userports.PushSendReport, error) {
	if s == nil || s.httpClient == nil {
		return userports.PushSendReport{}, errors.New("expo sender not configured")
	}
	if len(messages) == 0 {
		return userports.PushSendReport{}, nil
	}

	report := userports.PushSendReport{InvalidTokens: []string{}}

	// Expo recommends batches up to 100.
	for i := 0; i < len(messages); i += 100 {
		end := i + 100
		if end > len(messages) {
			end = len(messages)
		}
		batchReport, err := s.sendBatch(ctx, messages[i:end])
		if err != nil {
			return userports.PushSendReport{}, err
		}
		report.InvalidTokens = append(report.InvalidTokens, batchReport.InvalidTokens...)
	}
	return report, nil
}

type expoMessage struct {
	To    string         `json:"to"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data,omitempty"`
}

type expoResponse struct {
	Data   []expoTicket `json:"data"`
	Errors []expoError  `json:"errors"`
	Err    string       `json:"error"`
}

type expoError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type expoTicket struct {
	Status  string         `json:"status"`
	ID      string         `json:"id,omitempty"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (s *ExpoSender) sendBatch(ctx context.Context, batch []userports.PushMessage) (userports.PushSendReport, error) {
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		rep, err := s.sendBatchOnce(ctx, batch)
		if err == nil {
			return rep, nil
		}
		lastErr = err

		var httpErr *expoHTTPError
		if !errors.As(err, &httpErr) {
			break
		}
		if !httpErr.retryable {
			break
		}
		if attempt == s.maxRetries {
			break
		}

		d := httpErr.retryAfter
		if d <= 0 {
			d = s.backoff(attempt)
		}
		if d > 0 {
			if err := sleepWithContext(ctx, d); err != nil {
				return userports.PushSendReport{}, err
			}
		}
	}

	return userports.PushSendReport{}, lastErr
}

func (s *ExpoSender) sendBatchOnce(ctx context.Context, batch []userports.PushMessage) (userports.PushSendReport, error) {
	payload := make([]expoMessage, 0, len(batch))
	for _, m := range batch {
		payload = append(payload, expoMessage{To: m.To, Title: m.Title, Body: m.Body, Data: m.Data})
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return userports.PushSendReport{}, fmt.Errorf("expo marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.pushURL, bytes.NewReader(b))
	if err != nil {
		return userports.PushSendReport{}, fmt.Errorf("expo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.accessToken)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return userports.PushSendReport{}, fmt.Errorf("expo send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return userports.PushSendReport{}, newExpoHTTPError(resp)
	}

	// Decode tickets (provider returns per-message status in the same order).
	var r expoResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return userports.PushSendReport{}, fmt.Errorf("expo decode: %w", err)
	}
	if r.Err != "" {
		return userports.PushSendReport{}, fmt.Errorf("expo send: %s", r.Err)
	}
	if len(r.Errors) > 0 {
		// Some responses include top-level errors array.
		return userports.PushSendReport{}, fmt.Errorf("expo send: %s", r.Errors[0].Message)
	}

	report := userports.PushSendReport{InvalidTokens: []string{}}
	for i, t := range r.Data {
		if strings.EqualFold(t.Status, "ok") {
			continue
		}
		// Only take action when the provider tells us the token is invalid.
		if !ticketIndicatesInvalidToken(t) {
			continue
		}
		if i >= 0 && i < len(batch) {
			if tok := strings.TrimSpace(batch[i].To); tok != "" {
				report.InvalidTokens = append(report.InvalidTokens, tok)
			}
		}
	}

	return report, nil
}

func ticketIndicatesInvalidToken(t expoTicket) bool {
	if t.Details != nil {
		if v, ok := t.Details["error"]; ok {
			if s, ok := v.(string); ok {
				// Common invalid-token signals.
				s = strings.TrimSpace(s)
				switch s {
				case "DeviceNotRegistered", "InvalidPushToken", "InvalidCredentials":
					return true
				}
			}
		}
	}
	// Fallback heuristic: message contains invalid-token indicator.
	msg := strings.ToLower(t.Message)
	return strings.Contains(msg, "devicenotregistered") || strings.Contains(msg, "invalidpushtoken")
}

type expoHTTPError struct {
	statusCode int
	retryable  bool
	retryAfter time.Duration
}

func (e *expoHTTPError) Error() string {
	if e.retryAfter > 0 {
		return fmt.Sprintf("expo send: status %d (retry-after %s)", e.statusCode, e.retryAfter)
	}
	return fmt.Sprintf("expo send: status %d", e.statusCode)
}

func newExpoHTTPError(resp *http.Response) error {
	if resp == nil {
		return &expoHTTPError{statusCode: 0, retryable: false}
	}
	status := resp.StatusCode
	retryable := status == http.StatusTooManyRequests || status >= 500
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	return &expoHTTPError{statusCode: status, retryable: retryable, retryAfter: retryAfter}
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func (s *ExpoSender) backoff(attempt int) time.Duration {
	// attempt starts at 0.
	if attempt < 0 {
		attempt = 0
	}
	base := float64(s.minBackoff)
	mult := math.Pow(2, float64(attempt))
	d := time.Duration(base * mult)
	if d > s.maxBackoff {
		d = s.maxBackoff
	}
	// Add small jitter (0-25%) to reduce thundering herd.
	j := 1.0 + rand.Float64()*0.25
	d = time.Duration(float64(d) * j)
	if d < 0 {
		return 0
	}
	return d
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

var _ userports.PushSenderWithReport = (*ExpoSender)(nil)
