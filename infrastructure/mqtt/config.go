// Package mqtt provides MQTT/EMQX integration for live bus telemetry ingestion.
//
// This package implements the edge adapter for EMQX → Redis ingestion as defined
// in Phase 3 of the Alebus architecture. It uses the Eclipse Paho MQTT v5 client
// (autopaho) for connection management and manual acknowledgements.
//
// ARCHITECTURE:
//   - This is an INFRASTRUCTURE package (outermost layer)
//   - MQTT/EMQX concerns stay here; application/domain are unaware
//   - Wires into existing ports.LiveBusPublisher for Redis writes
//
// See: phase3.md, ADR-001_Live_Telemetry_Ingestion_Semantics_(Pre-EMQX).md
package mqtt

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the EMQX ingestor.
// Values are loaded from environment variables with sensible defaults.
type Config struct {
	// ─────────────────────────────────────────────────────────────────────────
	// MQTT Connection
	// ─────────────────────────────────────────────────────────────────────────

	// ServerURLs is a list of EMQX broker URLs (e.g., ["mqtt://host:1883"]).
	// Env: EMQX_SERVER_URLS (comma-separated)
	ServerURLs []string

	// ClientID is the unique MQTT client identifier.
	// Required for QoS1 session continuity across reconnects.
	// Env: EMQX_CLIENT_ID
	ClientID string

	// Username for EMQX authentication.
	// Env: EMQX_USERNAME
	Username string

	// Password for EMQX authentication.
	// Env: EMQX_PASSWORD
	Password string

	// TopicFilter is the base topic filter (e.g., "bus/+/gps").
	// Env: EMQX_TOPIC_FILTER
	TopicFilter string

	// SharedGroup is the shared subscription group name.
	// Final topic becomes: $share/{SharedGroup}/{TopicFilter}
	// Env: EMQX_SHARED_GROUP
	SharedGroup string

	// QoS is the MQTT Quality of Service level (0, 1, or 2).
	// Default: 1 (at-least-once delivery)
	// Env: EMQX_QOS
	QoS byte

	// KeepAliveSeconds is the MQTT keepalive interval.
	// Default: 30
	// Env: EMQX_KEEPALIVE_SECONDS
	KeepAliveSeconds uint16

	// CleanStartOnInitial controls whether to clear session on first connect.
	// Default: false (preserve session for QoS1 continuity)
	// Env: EMQX_CLEAN_START_ON_INITIAL
	CleanStartOnInitial bool

	// SessionExpirySec is how long the broker keeps session after disconnect.
	// Default: 300 (5 minutes)
	// Env: EMQX_SESSION_EXPIRY_SECONDS
	SessionExpirySec uint32

	// ConnectTimeout is the timeout for establishing MQTT connection.
	// Default: 10s
	// Env: EMQX_CONNECT_TIMEOUT
	ConnectTimeout time.Duration

	// ReconnectBackoffMin is the minimum backoff between reconnect attempts.
	// Default: 1s
	// Env: EMQX_RECONNECT_BACKOFF_MIN
	ReconnectBackoffMin time.Duration

	// ReconnectBackoffMax is the maximum backoff between reconnect attempts.
	// Default: 60s
	// Env: EMQX_RECONNECT_BACKOFF_MAX
	ReconnectBackoffMax time.Duration

	// ─────────────────────────────────────────────────────────────────────────
	// TLS (optional)
	// ─────────────────────────────────────────────────────────────────────────

	// TLSEnabled enables TLS for MQTT connections.
	// Env: EMQX_TLS_ENABLED
	TLSEnabled bool

	// TLSCAFile is the path to CA certificate for TLS connections.
	// Env: EMQX_TLS_CA_FILE
	TLSCAFile string

	// TLSCertFile is the path to client certificate for mTLS.
	// Env: EMQX_TLS_CERT_FILE
	TLSCertFile string

	// TLSKeyFile is the path to client private key for mTLS.
	// Env: EMQX_TLS_KEY_FILE
	TLSKeyFile string

	// TLSInsecureSkipVerify disables TLS certificate verification.
	// WARNING: Only for non-production testing.
	// Env: EMQX_TLS_INSECURE_SKIP_VERIFY
	TLSInsecureSkipVerify bool

	// ─────────────────────────────────────────────────────────────────────────
	// Ingestor Settings
	// ─────────────────────────────────────────────────────────────────────────

	// Workers is the number of concurrent worker goroutines.
	// Caps maximum concurrent Redis writes.
	// Default: 10
	// Env: INGESTOR_WORKERS
	Workers int

	// QueueSize is the maximum number of queued messages.
	// Default: 1000
	// Env: INGESTOR_QUEUE_SIZE
	QueueSize int

	// CoalesceEnabled enables latest-per-bus coalescing under overload.
	// Default: true
	// Env: INGESTOR_COALESCE_ENABLED
	CoalesceEnabled bool

	// DropPolicy determines behavior when queue is full.
	// Values: "drop_new" (reject new messages) or "coalesce_latest" (overwrite)
	// Default: "coalesce_latest"
	// Env: INGESTOR_DROP_POLICY
	DropPolicy string

	// ─────────────────────────────────────────────────────────────────────────
	// Retry Settings (for infra errors)
	// ─────────────────────────────────────────────────────────────────────────

	// NackBackoffMin is the minimum backoff for retrying failed publishes.
	// Default: 100ms
	// Env: NACK_BACKOFF_MIN
	NackBackoffMin time.Duration

	// NackBackoffMax is the maximum backoff for retrying failed publishes.
	// Default: 5s
	// Env: NACK_BACKOFF_MAX
	NackBackoffMax time.Duration

	// NackMaxRetries is the maximum retries before ACK+drop.
	// Default: 3
	// Env: NACK_MAX_RETRIES
	NackMaxRetries int

	// ─────────────────────────────────────────────────────────────────────────
	// Heartbeat
	// ─────────────────────────────────────────────────────────────────────────

	// HeartbeatKey is the Redis key for ingestion heartbeat.
	// Default: "ingestor:heartbeat"
	// Env: HEARTBEAT_KEY
	HeartbeatKey string

	// HeartbeatTTL is the TTL for the heartbeat key.
	// Default: 30s
	// Env: HEARTBEAT_TTL
	HeartbeatTTL time.Duration

	// ─────────────────────────────────────────────────────────────────────────
	// ACK Manager Settings
	// ─────────────────────────────────────────────────────────────────────────

	// PendingAckTimeout is the maximum time a pending ACK can wait.
	// After this, the pending ACK is evicted.
	// Default: 30s
	// Env: ACK_PENDING_TIMEOUT
	PendingAckTimeout time.Duration

	// MaxPendingAcks is the maximum number of pending ACKs.
	// Prevents unbounded memory growth.
	// Default: 10000
	// Env: ACK_MAX_PENDING
	MaxPendingAcks int

	// ─────────────────────────────────────────────────────────────────────────
	// HTTP Server
	// ─────────────────────────────────────────────────────────────────────────

	// HTTPAddr is the address for the metrics/health HTTP server.
	// Default: ":9100"
	// Env: INGESTOR_HTTP_ADDR
	HTTPAddr string

	// ─────────────────────────────────────────────────────────────────────────
	// GPS Enrichment (Phase 0 - Option B+)
	// ─────────────────────────────────────────────────────────────────────────

	// GPSEnrichmentEnabled enables GPS enrichment processing for raw GPS messages.
	// When false, GPS messages on bus/+/gps are ACK'd and dropped with metric.
	// When true, GPS messages are enriched and published to Redis.
	// Default: false (feature flag OFF)
	// Env: ENABLE_GPS_ENRICHMENT
	GPSEnrichmentEnabled bool

	// GPSLogEnabled enables structured logging for GPS enrichment.
	// Default: false (to avoid log spam)
	// Env: GPS_LOG_ENABLED
	GPSLogEnabled bool

	// GPSDebugEnabled enables extra debug diagnostics for stop-index projection.
	// When enabled, enrichment attaches projection/segment distances and worker logs
	// emit a detailed debug line.
	// Default: false
	// Env: GPS_DEBUG_ENABLED
	GPSDebugEnabled bool

	// ─────────────────────────────────────────────────────────────────────────
	// GPS Timestamp Windows (ADR-00X / Phase 5)
	// ─────────────────────────────────────────────────────────────────────────

	// GPSDeviceTimestampPastWindow is the maximum allowed age of device timestamp
	// compared to server receive time before it is treated as a replay at the edge.
	// Default: 5m
	// Env: GPS_DEVICE_TS_PAST_WINDOW
	GPSDeviceTimestampPastWindow time.Duration

	// GPSDeviceTimestampFutureWindow is the soft future skew window allowed for
	// device timestamps. Used for clamping/ordering logic downstream.
	// Default: 30s
	// Env: GPS_DEVICE_TS_FUTURE_WINDOW
	GPSDeviceTimestampFutureWindow time.Duration

	// GPSDeviceTimestampHardFutureLimit is the hard upper bound for device
	// timestamps in the future relative to server receive time. Beyond this,
	// messages are ACK'd and dropped.
	// Default: 5m
	// Env: GPS_DEVICE_TS_HARD_FUTURE_LIMIT
	GPSDeviceTimestampHardFutureLimit time.Duration

	// ─────────────────────────────────────────────────────────────────────────
	// GPS Enrichment - Staged Rollout (Phase 8 - Option B+)
	// ─────────────────────────────────────────────────────────────────────────

	// GPSAllowedBusPrefixes limits GPS enrichment to buses with IDs matching these prefixes.
	// If empty (default), all buses are allowed when GPSEnrichmentEnabled=true.
	// Use for staged rollout: start with specific bus prefixes, then expand.
	// Example: "BUS-001,BUS-002" enables enrichment only for these specific buses.
	// Example: "ROUTE-A-" enables enrichment for all buses starting with "ROUTE-A-".
	// Env: GPS_ALLOWED_BUS_PREFIXES (comma-separated)
	GPSAllowedBusPrefixes []string

	// GPSRolloutPercentage limits GPS enrichment to a percentage of buses (0-100).
	// Uses deterministic hashing of bus ID for consistent selection.
	// When 0 (default), percentage filtering is disabled (uses prefix filtering or all buses).
	// When 100, all buses are allowed (same as disabled).
	// Example: 10 means ~10% of buses get GPS enrichment (deterministic by bus ID hash).
	// Env: GPS_ROLLOUT_PERCENTAGE
	GPSRolloutPercentage int
}

// LoadConfig loads configuration from environment variables with defaults.
func LoadConfig() Config {
	cfg := Config{
		// MQTT defaults
		ServerURLs:          parseStringSlice(getEnv("EMQX_SERVER_URLS", "mqtt://localhost:1883")),
		ClientID:            getEnv("EMQX_CLIENT_ID", ""),
		Username:            getEnv("EMQX_USERNAME", ""),
		Password:            getEnv("EMQX_PASSWORD", ""),
		TopicFilter:         getEnv("EMQX_TOPIC_FILTER", "bus/+/gps"),
		SharedGroup:         getEnv("EMQX_SHARED_GROUP", "alebus-ingestor"),
		QoS:                 byte(getEnvInt("EMQX_QOS", 1)),
		KeepAliveSeconds:    uint16(getEnvInt("EMQX_KEEPALIVE_SECONDS", 30)),
		CleanStartOnInitial: getEnvBool("EMQX_CLEAN_START_ON_INITIAL", false),
		SessionExpirySec:    uint32(getEnvInt("EMQX_SESSION_EXPIRY_SECONDS", 300)),
		ConnectTimeout:      getEnvDuration("EMQX_CONNECT_TIMEOUT", 10*time.Second),
		ReconnectBackoffMin: getEnvDuration("EMQX_RECONNECT_BACKOFF_MIN", 1*time.Second),
		ReconnectBackoffMax: getEnvDuration("EMQX_RECONNECT_BACKOFF_MAX", 60*time.Second),

		// TLS defaults
		TLSEnabled:            getEnvBool("EMQX_TLS_ENABLED", false),
		TLSCAFile:             getEnv("EMQX_TLS_CA_FILE", ""),
		TLSCertFile:           getEnv("EMQX_TLS_CERT_FILE", ""),
		TLSKeyFile:            getEnv("EMQX_TLS_KEY_FILE", ""),
		TLSInsecureSkipVerify: getEnvBool("EMQX_TLS_INSECURE_SKIP_VERIFY", false),

		// Ingestor defaults
		Workers:         getEnvInt("INGESTOR_WORKERS", 10),
		QueueSize:       getEnvInt("INGESTOR_QUEUE_SIZE", 1000),
		CoalesceEnabled: getEnvBool("INGESTOR_COALESCE_ENABLED", true),
		DropPolicy:      getEnv("INGESTOR_DROP_POLICY", "coalesce_latest"),

		// Retry defaults
		NackBackoffMin: getEnvDuration("NACK_BACKOFF_MIN", 100*time.Millisecond),
		NackBackoffMax: getEnvDuration("NACK_BACKOFF_MAX", 5*time.Second),
		NackMaxRetries: getEnvInt("NACK_MAX_RETRIES", 3),

		// Heartbeat defaults
		HeartbeatKey: getEnv("HEARTBEAT_KEY", "ingestor:heartbeat"),
		HeartbeatTTL: getEnvDuration("HEARTBEAT_TTL", 30*time.Second),

		// ACK manager defaults
		PendingAckTimeout: getEnvDuration("ACK_PENDING_TIMEOUT", 30*time.Second),
		MaxPendingAcks:    getEnvInt("ACK_MAX_PENDING", 10000),

		// HTTP defaults
		HTTPAddr: getEnv("INGESTOR_HTTP_ADDR", ":9100"),

		// GPS Enrichment defaults (Phase 0 - Option B+)
		GPSEnrichmentEnabled: getEnvBool("ENABLE_GPS_ENRICHMENT", false),
		GPSLogEnabled:        getEnvBool("GPS_LOG_ENABLED", false),
		GPSDebugEnabled:      getEnvBool("GPS_DEBUG_ENABLED", false),

		// GPS Timestamp Windows (ADR-00X / Phase 5)
		GPSDeviceTimestampPastWindow:      getEnvDuration("GPS_DEVICE_TS_PAST_WINDOW", 5*time.Minute),
		GPSDeviceTimestampFutureWindow:    getEnvDuration("GPS_DEVICE_TS_FUTURE_WINDOW", 30*time.Second),
		GPSDeviceTimestampHardFutureLimit: getEnvDuration("GPS_DEVICE_TS_HARD_FUTURE_LIMIT", 5*time.Minute),

		// GPS Enrichment - Staged Rollout (Phase 8 - Option B+)
		GPSAllowedBusPrefixes: parseStringSlice(getEnv("GPS_ALLOWED_BUS_PREFIXES", "")),
		GPSRolloutPercentage:  getEnvInt("GPS_ROLLOUT_PERCENTAGE", 0),
	}

	return cfg
}

// Validate checks the configuration for required values and valid ranges.
// Returns an error describing the first validation failure, or nil if valid.
func (c *Config) Validate() error {
	// Required fields
	if len(c.ServerURLs) == 0 {
		return fmt.Errorf("EMQX_SERVER_URLS is required")
	}
	for i, u := range c.ServerURLs {
		if _, err := url.Parse(u); err != nil {
			return fmt.Errorf("EMQX_SERVER_URLS[%d] is invalid: %w", i, err)
		}
	}
	if c.ClientID == "" {
		return fmt.Errorf("EMQX_CLIENT_ID is required for QoS1 session continuity")
	}
	if c.TopicFilter == "" {
		return fmt.Errorf("EMQX_TOPIC_FILTER is required")
	}
	if c.SharedGroup == "" {
		return fmt.Errorf("EMQX_SHARED_GROUP is required for horizontal scaling")
	}

	// Range validations
	if c.QoS > 2 {
		return fmt.Errorf("EMQX_QOS must be 0, 1, or 2")
	}
	if c.Workers < 1 {
		return fmt.Errorf("INGESTOR_WORKERS must be >= 1")
	}
	if c.QueueSize < 1 {
		return fmt.Errorf("INGESTOR_QUEUE_SIZE must be >= 1")
	}
	if c.DropPolicy != "drop_new" && c.DropPolicy != "coalesce_latest" {
		return fmt.Errorf("INGESTOR_DROP_POLICY must be 'drop_new' or 'coalesce_latest'")
	}
	if c.NackMaxRetries < 0 {
		return fmt.Errorf("NACK_MAX_RETRIES must be >= 0")
	}
	if c.HeartbeatTTL <= 0 {
		return fmt.Errorf("HEARTBEAT_TTL must be > 0")
	}

	// GPS Rollout validations
	if c.GPSRolloutPercentage < 0 || c.GPSRolloutPercentage > 100 {
		return fmt.Errorf("GPS_ROLLOUT_PERCENTAGE must be between 0 and 100")
	}

	// GPS timestamp window validations
	if c.GPSDeviceTimestampPastWindow < 0 {
		return fmt.Errorf("GPS_DEVICE_TS_PAST_WINDOW must be >= 0")
	}
	if c.GPSDeviceTimestampFutureWindow < 0 {
		return fmt.Errorf("GPS_DEVICE_TS_FUTURE_WINDOW must be >= 0")
	}
	if c.GPSDeviceTimestampHardFutureLimit < 0 {
		return fmt.Errorf("GPS_DEVICE_TS_HARD_FUTURE_LIMIT must be >= 0")
	}
	if c.GPSDeviceTimestampHardFutureLimit > 0 && c.GPSDeviceTimestampFutureWindow > 0 && c.GPSDeviceTimestampHardFutureLimit < c.GPSDeviceTimestampFutureWindow {
		return fmt.Errorf("GPS_DEVICE_TS_HARD_FUTURE_LIMIT must be >= GPS_DEVICE_TS_FUTURE_WINDOW")
	}

	return nil
}

// SharedSubscriptionTopic returns the full shared subscription topic string.
// Format: $share/{SharedGroup}/{TopicFilter}
// DEPRECATED: Use SharedSubscriptionTopics() for multiple topic support.
func (c *Config) SharedSubscriptionTopic() string {
	return fmt.Sprintf("$share/%s/%s", c.SharedGroup, c.TopicFilter)
}

// SharedSubscriptionTopics returns all shared subscription topics.
// Splits TopicFilter by comma and returns each as a separate shared subscription.
// Format: $share/{SharedGroup}/{topic} for each topic in the filter.
func (c *Config) SharedSubscriptionTopics() []string {
	topics := strings.Split(c.TopicFilter, ",")
	result := make([]string, len(topics))
	for i, topic := range topics {
		result[i] = fmt.Sprintf("$share/%s/%s", c.SharedGroup, strings.TrimSpace(topic))
	}
	return result
}

// ParsedServerURLs returns the server URLs as parsed *url.URL objects.
func (c *Config) ParsedServerURLs() ([]*url.URL, error) {
	urls := make([]*url.URL, len(c.ServerURLs))
	for i, s := range c.ServerURLs {
		u, err := url.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid server URL %q: %w", s, err)
		}
		urls[i] = u
	}
	return urls, nil
}

// IsBusAllowedForGPSEnrichment checks if a bus is allowed for GPS enrichment
// based on staged rollout configuration (prefix filtering and percentage rollout).
// Returns true if:
//   - GPSEnrichmentEnabled is true AND
//   - Bus ID matches one of GPSAllowedBusPrefixes (if configured) AND
//   - Bus ID hash falls within GPSRolloutPercentage (if configured > 0)
func (c *Config) IsBusAllowedForGPSEnrichment(busID string) bool {
	if !c.GPSEnrichmentEnabled {
		return false
	}

	// Check prefix filter (if configured)
	if len(c.GPSAllowedBusPrefixes) > 0 {
		prefixMatch := false
		for _, prefix := range c.GPSAllowedBusPrefixes {
			if strings.HasPrefix(busID, prefix) {
				prefixMatch = true
				break
			}
		}
		if !prefixMatch {
			return false
		}
	}

	// Check percentage rollout (if configured)
	if c.GPSRolloutPercentage > 0 && c.GPSRolloutPercentage < 100 {
		// Deterministic hash ensures same bus always gets same decision
		hash := hashBusID(busID)
		if hash%100 >= c.GPSRolloutPercentage {
			return false
		}
	}

	return true
}

// hashBusID returns a deterministic hash of the bus ID for percentage rollout.
// Uses FNV-1a for fast, deterministic hashing.
func hashBusID(busID string) int {
	// FNV-1a hash
	const (
		fnvPrime  = 16777619
		fnvOffset = 2166136261
	)
	hash := uint32(fnvOffset)
	for i := 0; i < len(busID); i++ {
		hash ^= uint32(busID[i])
		hash *= fnvPrime
	}
	return int(hash % 100)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper functions for environment variable parsing
// ─────────────────────────────────────────────────────────────────────────────

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func parseStringSlice(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
