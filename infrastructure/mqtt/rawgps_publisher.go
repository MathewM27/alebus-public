package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	journeyports "github.com/MathewM27/busTrack-alebus/application/journey/ports"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

// RawGPSPublisher publishes raw GPS updates to the ingestion topic bus/{bus_id}/gps.
//
// This is an infrastructure adapter used by dev tooling (e.g., cmd/api simulate endpoints)
// to push telemetry into EMQX, which is then consumed by the ingestor for enrichment.
//
// QoS: 1 (at-least-once) to match ingestion semantics.
//
// Note: This publisher is intentionally small and independent from the ingestor's
// MQTT client (which focuses on subscriptions + manual ACKs).
//
// Implements: application/journey/ports.RawGPSPublisher.
// Provides: Close() for graceful shutdown.
//
// Not concurrency-heavy; safe for concurrent PublishRawGPS calls.
type RawGPSPublisher struct {
	brokerURL string
	clientID  string
	username  string
	password  string

	cm       *autopaho.ConnectionManager
	cmCancel context.CancelFunc

	mu         sync.RWMutex
	connected  bool
	lastErr    string
	lastChange time.Time
}

// Status returns the last-known connection status.
//
// This is intended for dev diagnostics only (e.g. /api/v1/mqtt/status).
func (p *RawGPSPublisher) Status() (connected bool, lastErr string, lastChange time.Time) {
	if p == nil {
		return false, "nil publisher", time.Time{}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connected, p.lastErr, p.lastChange
}

func NewRawGPSPublisher(brokerURL, clientID, username, password string) *RawGPSPublisher {
	return &RawGPSPublisher{brokerURL: brokerURL, clientID: clientID, username: username, password: password}
}

func (p *RawGPSPublisher) Connect(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("nil publisher")
	}
	if p.brokerURL == "" {
		return fmt.Errorf("missing broker URL")
	}
	if p.clientID == "" {
		return fmt.Errorf("missing client ID")
	}

	serverURL, err := url.Parse(p.brokerURL)
	if err != nil {
		return fmt.Errorf("invalid MQTT broker URL: %w", err)
	}

	cfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{serverURL},
		KeepAlive:                     30,
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         0,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connack *paho.Connack) {
			p.mu.Lock()
			p.connected = true
			p.lastErr = ""
			p.lastChange = time.Now().UTC()
			p.mu.Unlock()
		},
		OnConnectError: func(err error) {
			p.mu.Lock()
			p.connected = false
			if err != nil {
				p.lastErr = err.Error()
			}
			p.lastChange = time.Now().UTC()
			p.mu.Unlock()
		},
		ConnectUsername: p.username,
		ConnectPassword: []byte(p.password),
		ClientConfig: paho.ClientConfig{
			ClientID: p.clientID,
			OnServerDisconnect: func(d *paho.Disconnect) {
				p.mu.Lock()
				p.connected = false
				if d != nil {
					p.lastErr = fmt.Sprintf("server disconnect: reason=%d", d.ReasonCode)
				} else {
					p.lastErr = "server disconnect"
				}
				p.lastChange = time.Now().UTC()
				p.mu.Unlock()
			},
		},
	}

	// IMPORTANT: autopaho ties the connection manager lifecycle to the context passed
	// into NewConnection. Do NOT use a request-scoped or timeout context here, or the
	// manager will shut down immediately after Connect() returns.
	cmCtx := context.Background()
	if p.cmCancel == nil {
		var cancel context.CancelFunc
		cmCtx, cancel = context.WithCancel(context.Background())
		p.cmCancel = cancel
	}

	cm, err := autopaho.NewConnection(cmCtx, cfg)
	if err != nil {
		p.mu.Lock()
		p.connected = false
		p.lastErr = err.Error()
		p.lastChange = time.Now().UTC()
		p.mu.Unlock()
		return fmt.Errorf("failed to create MQTT connection: %w", err)
	}
	p.cm = cm

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := cm.AwaitConnection(connCtx); err != nil {
		p.mu.Lock()
		p.connected = false
		p.lastErr = err.Error()
		p.lastChange = time.Now().UTC()
		p.mu.Unlock()
		return fmt.Errorf("MQTT connection timeout: %w", err)
	}

	return nil
}

func (p *RawGPSPublisher) Close() error {
	if p == nil || p.cm == nil {
		return nil
	}
	if p.cmCancel != nil {
		p.cmCancel()
		p.cmCancel = nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.cm.Disconnect(ctx)

	p.mu.Lock()
	p.connected = false
	p.lastErr = "closed"
	p.lastChange = time.Now().UTC()
	p.mu.Unlock()
	return nil
}

func (p *RawGPSPublisher) PublishRawGPS(ctx context.Context, update journeyports.RawGPSUpdate) error {
	if p == nil || p.cm == nil {
		return fmt.Errorf("MQTT publisher not connected")
	}
	// autopaho can reconnect automatically; avoid racing the reconnect window.
	connCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := p.cm.AwaitConnection(connCtx); err != nil {
		return fmt.Errorf("MQTT publisher not connected: %w", err)
	}

	if err := update.Validate(); err != nil {
		return err
	}

	topic := fmt.Sprintf("bus/%s/gps", update.BusID)

	payload := struct {
		BusID       string  `json:"bus_id"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		TimestampMs int64   `json:"timestamp_ms"`
		SpeedKmh    float64 `json:"speed_kmh,omitempty"`
		Heading     float64 `json:"heading,omitempty"`
		AccuracyM   float64 `json:"accuracy_m,omitempty"`
	}{
		BusID:       update.BusID,
		Lat:         update.Lat,
		Lon:         update.Lon,
		TimestampMs: update.Timestamp.UTC().UnixMilli(),
		SpeedKmh:    update.SpeedKmh,
		Heading:     update.Heading,
		AccuracyM:   update.AccuracyM,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal raw GPS: %w", err)
	}

	publish := func() error {
		_, err := p.cm.Publish(ctx, &paho.Publish{
			Topic:   topic,
			QoS:     1,
			Payload: b,
		})
		return err
	}

	err = publish()
	if err != nil {
		// Common transient error during reconnect.
		if strings.Contains(strings.ToLower(err.Error()), "no connection available") {
			retryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if waitErr := p.cm.AwaitConnection(retryCtx); waitErr == nil {
				err = publish()
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to publish raw GPS: %w", err)
	}

	return nil
}

var _ journeyports.RawGPSPublisher = (*RawGPSPublisher)(nil)
