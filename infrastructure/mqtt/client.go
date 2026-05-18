package mqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

// ─────────────────────────────────────────────────────────────────────────────
// MQTT Client Adapter — autopaho Wrapper
// ─────────────────────────────────────────────────────────────────────────────
//
// Per Phase 3 blueprint:
//   - Uses autopaho for automatic reconnection with backoff
//   - Shared subscription: $share/{group}/{topicFilter}
//   - Manual acknowledgement mode (EnableManualAcknowledgment=true)
//   - Connection-scoped ACK manager (reset on reconnect)
//   - Validates shared subscription support in CONNACK
//
// ─────────────────────────────────────────────────────────────────────────────

// ErrSharedSubNotSupported indicates the broker doesn't support shared subscriptions.
var ErrSharedSubNotSupported = errors.New("broker does not support shared subscriptions")

// ErrClientStopped indicates the client has been stopped.
var ErrClientStopped = errors.New("mqtt client stopped")

// MessageHandler is called for each received message.
// The handler should process the message and return quickly.
// The returned RecvSeq should be stored for later ACK decision reporting.
type MessageHandler func(ctx context.Context, msg *ReceivedMessage)

// ReceivedMessage represents a message received from MQTT.
type ReceivedMessage struct {
	// Topic is the MQTT topic the message was received on.
	Topic string

	// Payload is the raw message payload.
	Payload []byte

	// RecvSeq is the sequence number assigned by the ACK manager.
	RecvSeq uint64

	// ReceivedAt is when the message was received.
	ReceivedAt time.Time

	// QoS is the message QoS level.
	QoS byte

	// PacketID is the MQTT packet ID (for QoS > 0).
	PacketID uint16
}

// ConnectionState represents the current connection state.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	default:
		return fmt.Sprintf("ConnectionState(%d)", s)
	}
}

// MQTTClient wraps autopaho for MQTT connectivity with manual ACK support.
type MQTTClient struct {
	config Config
	cm     *autopaho.ConnectionManager

	// Message handling
	handler MessageHandler
	ackMgr  *AckManager

	// State
	mu              sync.RWMutex
	state           ConnectionState
	lastConnectTime time.Time
	lastError       error
	stopped         bool
	stopCh          chan struct{}

	// Metrics
	messagesReceived atomic.Uint64
	reconnectCount   atomic.Uint64
}

// NewMQTTClient creates a new MQTT client with the given configuration.
//
// The client is not connected until Start() is called.
func NewMQTTClient(cfg Config, handler MessageHandler) (*MQTTClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if handler == nil {
		return nil, errors.New("message handler is required")
	}

	client := &MQTTClient{
		config:  cfg,
		handler: handler,
		state:   StateDisconnected,
		stopCh:  make(chan struct{}),
	}

	return client, nil
}

// Start connects to the MQTT broker and begins receiving messages.
//
// This method blocks until the initial connection is established or the
// context is cancelled. After the initial connection, autopaho handles
// reconnection automatically.
func (c *MQTTClient) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return ErrClientStopped
	}
	c.state = StateConnecting
	c.mu.Unlock()

	// Parse server URLs
	serverURLs, err := c.config.ParsedServerURLs()
	if err != nil {
		return fmt.Errorf("invalid server URLs: %w", err)
	}

	// Create ACK manager for this connection
	c.ackMgr = NewAckManager(AckManagerConfig{
		PendingTimeout: c.config.PendingAckTimeout,
		MaxPendingSize: c.config.MaxPendingAcks,
	})

	// Build autopaho configuration
	pahoCfg := c.buildAutopahoConfig(serverURLs)

	// Create connection manager
	cm, err := autopaho.NewConnection(ctx, pahoCfg)
	if err != nil {
		c.mu.Lock()
		c.state = StateDisconnected
		c.lastError = err
		c.mu.Unlock()
		return fmt.Errorf("failed to create connection: %w", err)
	}

	c.cm = cm

	// Wait for initial connection
	if err := cm.AwaitConnection(ctx); err != nil {
		c.mu.Lock()
		c.state = StateDisconnected
		c.lastError = err
		c.mu.Unlock()
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Start ACK manager flush loop
	go c.ackMgr.FlushLoop(ctx)

	c.mu.Lock()
	c.state = StateConnected
	c.lastConnectTime = time.Now()
	c.mu.Unlock()

	return nil
}

// Stop disconnects from the broker and stops the client.
//
// Any pending messages will not be acknowledged (broker will redeliver
// on reconnect to another instance).
func (c *MQTTClient) Stop(ctx context.Context) error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.stopped = true
	close(c.stopCh)
	c.mu.Unlock()

	// Stop ACK manager first (prevents late ACKs)
	if c.ackMgr != nil {
		c.ackMgr.Stop()
	}

	// Disconnect from broker
	if c.cm != nil {
		disconnectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := c.cm.Disconnect(disconnectCtx); err != nil {
			// Log but don't fail — we're shutting down anyway
			c.mu.Lock()
			c.lastError = err
			c.mu.Unlock()
		}
	}

	c.mu.Lock()
	c.state = StateDisconnected
	c.mu.Unlock()

	return nil
}

// AckMessage acknowledges a message with the given decision.
//
// This should be called after processing a message to report the ACK decision.
// The ACK manager will send the actual PUBACK in order.
func (c *MQTTClient) AckMessage(recvSeq uint64, decision AckDecision) bool {
	if c.ackMgr == nil {
		return false
	}
	return c.ackMgr.Complete(recvSeq, decision)
}

// UpdateRetryToAck updates a retry decision to ACK after successful retry.
func (c *MQTTClient) UpdateRetryToAck(recvSeq uint64) bool {
	if c.ackMgr == nil {
		return false
	}
	return c.ackMgr.UpdateRetryToAck(recvSeq)
}

// State returns the current connection state.
func (c *MQTTClient) State() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// LastError returns the last error encountered.
func (c *MQTTClient) LastError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

// Stats returns client statistics.
type MQTTClientStats struct {
	State            ConnectionState
	MessagesReceived uint64
	ReconnectCount   uint64
	PendingAcks      int
	LastConnectTime  time.Time
}

// Stats returns current statistics.
func (c *MQTTClient) Stats() MQTTClientStats {
	c.mu.RLock()
	state := c.state
	lastConnect := c.lastConnectTime
	c.mu.RUnlock()

	pendingAcks := 0
	if c.ackMgr != nil {
		pendingAcks = c.ackMgr.PendingCount()
	}

	return MQTTClientStats{
		State:            state,
		MessagesReceived: c.messagesReceived.Load(),
		ReconnectCount:   c.reconnectCount.Load(),
		PendingAcks:      pendingAcks,
		LastConnectTime:  lastConnect,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal: autopaho configuration
// ─────────────────────────────────────────────────────────────────────────────

func (c *MQTTClient) buildAutopahoConfig(serverURLs []*url.URL) autopaho.ClientConfig {
	reconnectBackoff := func(attempt int) time.Duration {
		if attempt < 1 {
			attempt = 1
		}
		min := c.config.ReconnectBackoffMin
		max := c.config.ReconnectBackoffMax
		if min <= 0 {
			min = 10 * time.Second
		}
		if max <= 0 {
			max = min
		}
		backoff := min * time.Duration(1<<uint(attempt-1))
		if backoff > max {
			backoff = max
		}
		return backoff
	}

	cfg := autopaho.ClientConfig{
		ServerUrls:                    serverURLs,
		KeepAlive:                     c.config.KeepAliveSeconds,
		CleanStartOnInitialConnection: c.config.CleanStartOnInitial,
		SessionExpiryInterval:         c.config.SessionExpirySec,
		ReconnectBackoff:              reconnectBackoff,
		ConnectTimeout:                c.config.ConnectTimeout,

		// Optional authentication (required when EMQX disallows anonymous clients).
		ConnectUsername: c.config.Username,
		ConnectPassword: []byte(c.config.Password),

		OnConnectionUp: c.onConnectionUp,
		OnConnectError: c.onConnectError,

		ClientConfig: paho.ClientConfig{
			ClientID: c.config.ClientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				c.onPublishReceived,
			},
			OnClientError: c.onClientError,
			OnServerDisconnect: func(d *paho.Disconnect) {
				c.onServerDisconnect(d)
			},
			EnableManualAcknowledgment: true, // Critical for ordered ACKs
		},
	}

	// Add TLS configuration if enabled
	if tlsCfg := c.buildTLSConfig(); tlsCfg != nil {
		cfg.TlsCfg = tlsCfg
	}

	return cfg
}

// onConnectionUp is called when connection is established.
func (c *MQTTClient) onConnectionUp(cm *autopaho.ConnectionManager, connack *paho.Connack) {
	c.mu.Lock()
	wasConnected := c.state == StateConnected
	c.state = StateConnected
	c.lastConnectTime = time.Now()
	c.mu.Unlock()

	if wasConnected {
		c.reconnectCount.Add(1)
		// Reset ACK manager on reconnect (connection-scoped)
		if c.ackMgr != nil {
			c.ackMgr.Stop()
		}
		c.ackMgr = NewAckManager(AckManagerConfig{
			PendingTimeout: c.config.PendingAckTimeout,
			MaxPendingSize: c.config.MaxPendingAcks,
		})
		// Start new flush loop (context from cm is not available here,
		// so we use a background context that stops when ackMgr is stopped)
		go c.ackMgr.FlushLoop(context.Background())
	}

	// Check shared subscription support
	// Note: SharedSubAvailable defaults to true if not explicitly set by broker
	if connack.Properties != nil && !connack.Properties.SharedSubAvailable {
		// Broker explicitly doesn't support shared subscriptions — this is fatal
		c.mu.Lock()
		c.lastError = ErrSharedSubNotSupported
		c.mu.Unlock()
		// Note: In production, we might want to disconnect here
		return
	}

	// Subscribe to shared topic
	// Handle multiple topics (comma-separated in config)
	topics := c.config.SharedSubscriptionTopics()
	subscriptions := make([]paho.SubscribeOptions, len(topics))
	for i, topic := range topics {
		subscriptions[i] = paho.SubscribeOptions{
			Topic: topic,
			QoS:   c.config.QoS,
		}
	}
	_, err := cm.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: subscriptions,
	})

	if err != nil {
		c.mu.Lock()
		c.lastError = fmt.Errorf("subscribe failed: %w", err)
		c.mu.Unlock()
	}
}

// onConnectError is called when connection fails.
func (c *MQTTClient) onConnectError(err error) {
	c.mu.Lock()
	c.state = StateConnecting
	c.lastError = err
	c.mu.Unlock()
}

// onPublishReceived handles incoming messages.
func (c *MQTTClient) onPublishReceived(pr paho.PublishReceived) (bool, error) {
	// Check if stopped
	select {
	case <-c.stopCh:
		return true, nil // Don't process, ACK to clear
	default:
	}

	c.messagesReceived.Add(1)

	// Register with ACK manager to get sequence number
	// Create ACK function that will be called when it's this message's turn
	// We capture the Client and Packet for the ACK call
	packet := pr.Packet
	client := pr.Client
	ackFn := func() error {
		if client != nil && packet != nil {
			return client.Ack(packet)
		}
		return nil
	}

	recvSeq, ok := c.ackMgr.Register(ackFn)
	if !ok {
		// ACK manager stopped, ACK immediately to clear
		return true, nil
	}

	// Build received message
	msg := &ReceivedMessage{
		Topic:      pr.Packet.Topic,
		Payload:    pr.Packet.Payload,
		RecvSeq:    recvSeq,
		ReceivedAt: time.Now(),
		QoS:        pr.Packet.QoS,
		PacketID:   pr.Packet.PacketID,
	}

	// Call handler (should be non-blocking, typically enqueues to queue)
	c.handler(context.Background(), msg)

	// Return false to indicate we're handling ACK manually
	return false, nil
}

// onClientError handles client errors.
func (c *MQTTClient) onClientError(err error) {
	c.mu.Lock()
	c.lastError = err
	c.mu.Unlock()
}

// onServerDisconnect handles server-initiated disconnect.
func (c *MQTTClient) onServerDisconnect(d *paho.Disconnect) {
	c.mu.Lock()
	c.state = StateDisconnected
	if d.Properties != nil && d.Properties.ReasonString != "" {
		c.lastError = fmt.Errorf("server disconnect: %s (code %d)", d.Properties.ReasonString, d.ReasonCode)
	} else {
		c.lastError = fmt.Errorf("server disconnect: code %d", d.ReasonCode)
	}
	c.mu.Unlock()

	// Stop current ACK manager (connection-scoped)
	if c.ackMgr != nil {
		c.ackMgr.Stop()
	}
}

// buildTLSConfig creates TLS configuration if enabled.
func (c *MQTTClient) buildTLSConfig() *tls.Config {
	if !c.config.TLSEnabled {
		return nil
	}

	return &tls.Config{
		InsecureSkipVerify: c.config.TLSInsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test Helpers
// ─────────────────────────────────────────────────────────────────────────────

// AckManager returns the ACK manager for testing purposes.
func (c *MQTTClient) AckManager() *AckManager {
	return c.ackMgr
}

// SetAckManager sets the ACK manager for testing purposes.
func (c *MQTTClient) SetAckManager(am *AckManager) {
	c.ackMgr = am
}

// InjectConnectionManager injects a connection manager for testing.
// This bypasses the normal connection flow.
func (c *MQTTClient) InjectConnectionManager(cm *autopaho.ConnectionManager) {
	c.cm = cm
}
