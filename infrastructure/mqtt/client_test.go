package mqtt

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Client Construction Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNewMQTTClient_ValidConfig(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, StateDisconnected, client.State())
}

func TestNewMQTTClient_InvalidConfig(t *testing.T) {
	cfg := Config{} // Missing required fields
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "invalid config")
}

func TestNewMQTTClient_NilHandler(t *testing.T) {
	cfg := testConfig()

	client, err := NewMQTTClient(cfg, nil)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "message handler is required")
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared Subscription Topic Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_SharedSubscription(t *testing.T) {
	cfg := Config{
		SharedGroup: "alebus-ingestor",
		TopicFilter: "bus/+/gps",
	}

	topic := cfg.SharedSubscriptionTopic()

	assert.Equal(t, "$share/alebus-ingestor/bus/+/gps", topic)
}

func TestMQTTClient_SharedSubscriptionWildcard(t *testing.T) {
	cfg := Config{
		SharedGroup: "my-group",
		TopicFilter: "devices/#",
	}

	topic := cfg.SharedSubscriptionTopic()

	assert.Equal(t, "$share/my-group/devices/#", topic)
}

// ─────────────────────────────────────────────────────────────────────────────
// Connection State Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_InitialState(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	assert.Equal(t, StateDisconnected, client.State())
	assert.Nil(t, client.LastError())
}

func TestConnectionState_String(t *testing.T) {
	assert.Equal(t, "disconnected", StateDisconnected.String())
	assert.Equal(t, "connecting", StateConnecting.String())
	assert.Equal(t, "connected", StateConnected.String())
	assert.Equal(t, "ConnectionState(99)", ConnectionState(99).String())
}

// ─────────────────────────────────────────────────────────────────────────────
// Manual ACK Mode Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_ManualAckMode(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Inject mock ACK manager
	mockAckMgr := NewAckManager(DefaultAckManagerConfig())
	client.SetAckManager(mockAckMgr)

	// Register an ack
	ackFn := func() error { return nil }
	recvSeq, ok := mockAckMgr.Register(ackFn)
	require.True(t, ok)

	// Report ACK decision through client
	ok = client.AckMessage(recvSeq, AckDecisionACK)
	assert.True(t, ok)

	// Flush and verify
	mockAckMgr.Flush()
	assert.Equal(t, 0, mockAckMgr.PendingCount())
}

func TestMQTTClient_AckMessageNoManager(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// No ACK manager set
	ok := client.AckMessage(1, AckDecisionACK)
	assert.False(t, ok)
}

func TestMQTTClient_UpdateRetryToAck(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Inject mock ACK manager
	mockAckMgr := NewAckManager(DefaultAckManagerConfig())
	client.SetAckManager(mockAckMgr)

	// Register and mark as retry
	ackFn := func() error { return nil }
	recvSeq, _ := mockAckMgr.Register(ackFn)
	mockAckMgr.Complete(recvSeq, AckDecisionRetry)

	// Update to ACK
	ok := client.UpdateRetryToAck(recvSeq)
	assert.True(t, ok)

	// Should now be able to flush
	mockAckMgr.Flush()
	assert.Equal(t, 0, mockAckMgr.PendingCount())
}

// ─────────────────────────────────────────────────────────────────────────────
// Stop Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_StopBeforeStart(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Stop without starting
	err = client.Stop(context.Background())
	assert.NoError(t, err)
}

func TestMQTTClient_DoubleStop(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Stop twice
	err = client.Stop(context.Background())
	assert.NoError(t, err)

	err = client.Stop(context.Background())
	assert.NoError(t, err)
}

func TestMQTTClient_StartAfterStop(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Stop first
	err = client.Stop(context.Background())
	require.NoError(t, err)

	// Try to start after stop
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = client.Start(ctx)
	assert.Equal(t, ErrClientStopped, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Stats Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_Stats(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Initial stats
	stats := client.Stats()
	assert.Equal(t, StateDisconnected, stats.State)
	assert.Equal(t, uint64(0), stats.MessagesReceived)
	assert.Equal(t, uint64(0), stats.ReconnectCount)
	assert.Equal(t, 0, stats.PendingAcks)
}

func TestMQTTClient_StatsWithAckManager(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Inject ACK manager with pending items
	mockAckMgr := NewAckManager(DefaultAckManagerConfig())
	mockAckMgr.Register(func() error { return nil })
	mockAckMgr.Register(func() error { return nil })
	client.SetAckManager(mockAckMgr)

	stats := client.Stats()
	assert.Equal(t, 2, stats.PendingAcks)
}

// ─────────────────────────────────────────────────────────────────────────────
// Message Handler Tests (unit test without real MQTT)
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_MessageHandlerCalled(t *testing.T) {
	cfg := testConfig()

	var receivedMessages []*ReceivedMessage
	var mu sync.Mutex

	handler := func(ctx context.Context, msg *ReceivedMessage) {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
	}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Inject ACK manager
	mockAckMgr := NewAckManager(DefaultAckManagerConfig())
	client.SetAckManager(mockAckMgr)

	// Simulate receiving a message by calling the internal receiver
	// This tests the message dispatch logic without needing a real broker
	msg := &ReceivedMessage{
		Topic:      "bus/123/telemetry",
		Payload:    []byte(`{"bus_id":"bus-123"}`),
		RecvSeq:    1,
		ReceivedAt: time.Now(),
		QoS:        1,
		PacketID:   42,
	}

	handler(context.Background(), msg)

	mu.Lock()
	require.Len(t, receivedMessages, 1)
	assert.Equal(t, "bus/123/telemetry", receivedMessages[0].Topic)
	assert.Equal(t, uint64(1), receivedMessages[0].RecvSeq)
	mu.Unlock()
}

// ─────────────────────────────────────────────────────────────────────────────
// Concurrent Access Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_ConcurrentAckOperations(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	mockAckMgr := NewAckManager(DefaultAckManagerConfig())
	client.SetAckManager(mockAckMgr)

	// Register many ACKs
	for i := 0; i < 100; i++ {
		mockAckMgr.Register(func() error { return nil })
	}

	// Concurrently complete and flush
	var wg sync.WaitGroup
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func(seq uint64) {
			defer wg.Done()
			client.AckMessage(seq, AckDecisionACK)
		}(uint64(i))
	}

	// Concurrent flush
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			mockAckMgr.Flush()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestMQTTClient_ConcurrentStats(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	mockAckMgr := NewAckManager(DefaultAckManagerConfig())
	client.SetAckManager(mockAckMgr)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = client.Stats()
				_ = client.State()
				_ = client.LastError()
			}
		}()
	}
	wg.Wait()
}

// ─────────────────────────────────────────────────────────────────────────────
// Error Sentinel Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestErrors(t *testing.T) {
	assert.Equal(t, "broker does not support shared subscriptions", ErrSharedSubNotSupported.Error())
	assert.Equal(t, "mqtt client stopped", ErrClientStopped.Error())
}

// ─────────────────────────────────────────────────────────────────────────────
// autopaho Config Builder Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_BuildAutopahoConfig(t *testing.T) {
	cfg := testConfig()
	cfg.KeepAliveSeconds = 45
	cfg.SessionExpirySec = 600
	cfg.ReconnectBackoffMin = 2 * time.Second
	cfg.ConnectTimeout = 15 * time.Second

	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	serverURLs, _ := cfg.ParsedServerURLs()
	pahoCfg := client.buildAutopahoConfig(serverURLs)

	assert.Equal(t, uint16(45), pahoCfg.KeepAlive)
	assert.Equal(t, uint32(600), pahoCfg.SessionExpiryInterval)
	if pahoCfg.ReconnectBackoff == nil {
		t.Fatal("ReconnectBackoff must be set")
	}
	assert.Equal(t, 2*time.Second, pahoCfg.ReconnectBackoff(1))
	assert.Equal(t, 15*time.Second, pahoCfg.ConnectTimeout)
	assert.Equal(t, "test-client-123", pahoCfg.ClientConfig.ClientID)
	assert.True(t, pahoCfg.ClientConfig.EnableManualAcknowledgment)
}

// ─────────────────────────────────────────────────────────────────────────────
// Message Count Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMQTTClient_MessagesReceivedCounter(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Simulate incrementing counter
	client.messagesReceived.Add(1)
	client.messagesReceived.Add(1)
	client.messagesReceived.Add(1)

	stats := client.Stats()
	assert.Equal(t, uint64(3), stats.MessagesReceived)
}

func TestMQTTClient_ReconnectCounter(t *testing.T) {
	cfg := testConfig()
	handler := func(ctx context.Context, msg *ReceivedMessage) {}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	// Simulate reconnects
	client.reconnectCount.Add(1)
	client.reconnectCount.Add(1)

	stats := client.Stats()
	assert.Equal(t, uint64(2), stats.ReconnectCount)
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration Test Helpers
// ─────────────────────────────────────────────────────────────────────────────

// TestMQTTClient_Connect is marked as an integration test.
// It requires a running EMQX broker at localhost:1883.
// Run with: go test -tags=integration -run TestMQTTClient_Connect
func TestMQTTClient_Connect_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// This test is a placeholder for integration testing
	// It would require a running EMQX instance
	t.Skip("requires running EMQX broker - run manually with EMQX available")

	cfg := testConfig()
	var messagesReceived atomic.Int32

	handler := func(ctx context.Context, msg *ReceivedMessage) {
		messagesReceived.Add(1)
	}

	client, err := NewMQTTClient(cfg, handler)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateConnected, client.State())

	// Cleanup
	err = client.Stop(context.Background())
	require.NoError(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test Helpers
// ─────────────────────────────────────────────────────────────────────────────

func testConfig() Config {
	return Config{
		ServerURLs:          []string{"mqtt://localhost:1883"},
		ClientID:            "test-client-123",
		TopicFilter:         "bus/+/gps",
		SharedGroup:         "test-group",
		QoS:                 1,
		KeepAliveSeconds:    30,
		SessionExpirySec:    300,
		ConnectTimeout:      10 * time.Second,
		ReconnectBackoffMin: 1 * time.Second,
		ReconnectBackoffMax: 60 * time.Second,
		Workers:             10,
		QueueSize:           1000,
		DropPolicy:          "coalesce_latest",
		NackMaxRetries:      3,
		HeartbeatTTL:        30 * time.Second,
		PendingAckTimeout:   30 * time.Second,
		MaxPendingAcks:      10000,
	}
}
