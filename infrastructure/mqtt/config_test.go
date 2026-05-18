package mqtt

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Config Loading Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestConfigFromEnv(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"EMQX_SERVER_URLS":            "mqtt://broker1:1883,mqtt://broker2:1883",
		"EMQX_CLIENT_ID":              "test-client-123",
		"EMQX_USERNAME":               "testuser",
		"EMQX_PASSWORD":               "testpass",
		"EMQX_TOPIC_FILTER":           "bus/+/gps",
		"EMQX_SHARED_GROUP":           "test-group",
		"EMQX_QOS":                    "1",
		"EMQX_KEEPALIVE_SECONDS":      "60",
		"EMQX_SESSION_EXPIRY_SECONDS": "600",
		"INGESTOR_WORKERS":            "20",
		"INGESTOR_QUEUE_SIZE":         "2000",
		"INGESTOR_COALESCE_ENABLED":   "false",
		"INGESTOR_DROP_POLICY":        "drop_new",
		"NACK_MAX_RETRIES":            "5",
		"HEARTBEAT_KEY":               "custom:heartbeat",
		"HEARTBEAT_TTL":               "45s",
		"INGESTOR_HTTP_ADDR":          ":8080",
	}

	// Set all env vars
	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cfg := LoadConfig()

	// Verify all values loaded correctly
	assert.Equal(t, []string{"mqtt://broker1:1883", "mqtt://broker2:1883"}, cfg.ServerURLs)
	assert.Equal(t, "test-client-123", cfg.ClientID)
	assert.Equal(t, "testuser", cfg.Username)
	assert.Equal(t, "testpass", cfg.Password)
	assert.Equal(t, "bus/+/gps", cfg.TopicFilter)
	assert.Equal(t, "test-group", cfg.SharedGroup)
	assert.Equal(t, byte(1), cfg.QoS)
	assert.Equal(t, uint16(60), cfg.KeepAliveSeconds)
	assert.Equal(t, uint32(600), cfg.SessionExpirySec)
	assert.Equal(t, 20, cfg.Workers)
	assert.Equal(t, 2000, cfg.QueueSize)
	assert.Equal(t, false, cfg.CoalesceEnabled)
	assert.Equal(t, "drop_new", cfg.DropPolicy)
	assert.Equal(t, 5, cfg.NackMaxRetries)
	assert.Equal(t, "custom:heartbeat", cfg.HeartbeatKey)
	assert.Equal(t, 45*time.Second, cfg.HeartbeatTTL)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
}

func TestConfigDefaults(t *testing.T) {
	// Clear all relevant env vars
	envVars := []string{
		"EMQX_SERVER_URLS", "EMQX_CLIENT_ID", "EMQX_USERNAME", "EMQX_PASSWORD",
		"EMQX_TOPIC_FILTER", "EMQX_SHARED_GROUP", "EMQX_QOS", "EMQX_KEEPALIVE_SECONDS",
		"INGESTOR_WORKERS", "INGESTOR_QUEUE_SIZE", "INGESTOR_COALESCE_ENABLED",
		"INGESTOR_DROP_POLICY", "NACK_MAX_RETRIES", "HEARTBEAT_KEY", "HEARTBEAT_TTL",
	}
	for _, k := range envVars {
		os.Unsetenv(k)
	}

	cfg := LoadConfig()

	// Verify defaults
	assert.Equal(t, []string{"mqtt://localhost:1883"}, cfg.ServerURLs)
	assert.Equal(t, "", cfg.ClientID) // No default - required
	assert.Equal(t, "bus/+/gps", cfg.TopicFilter)
	assert.Equal(t, "alebus-ingestor", cfg.SharedGroup)
	assert.Equal(t, byte(1), cfg.QoS)
	assert.Equal(t, uint16(30), cfg.KeepAliveSeconds)
	assert.Equal(t, uint32(300), cfg.SessionExpirySec)
	assert.Equal(t, 10, cfg.Workers)
	assert.Equal(t, 1000, cfg.QueueSize)
	assert.Equal(t, true, cfg.CoalesceEnabled)
	assert.Equal(t, "coalesce_latest", cfg.DropPolicy)
	assert.Equal(t, 3, cfg.NackMaxRetries)
	assert.Equal(t, "ingestor:heartbeat", cfg.HeartbeatKey)
	assert.Equal(t, 30*time.Second, cfg.HeartbeatTTL)
	assert.Equal(t, ":9100", cfg.HTTPAddr)
	assert.Equal(t, 10*time.Second, cfg.ConnectTimeout)
	assert.Equal(t, 1*time.Second, cfg.ReconnectBackoffMin)
	assert.Equal(t, 60*time.Second, cfg.ReconnectBackoffMax)
	assert.Equal(t, 100*time.Millisecond, cfg.NackBackoffMin)
	assert.Equal(t, 5*time.Second, cfg.NackBackoffMax)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{
			name: "valid config",
			modify: func(c *Config) {
				c.ClientID = "test-client"
			},
			wantErr: "",
		},
		{
			name: "missing client ID",
			modify: func(c *Config) {
				c.ClientID = ""
			},
			wantErr: "EMQX_CLIENT_ID is required",
		},
		{
			name: "empty server URLs",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.ServerURLs = nil
			},
			wantErr: "EMQX_SERVER_URLS is required",
		},
		{
			name: "missing topic filter",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.TopicFilter = ""
			},
			wantErr: "EMQX_TOPIC_FILTER is required",
		},
		{
			name: "missing shared group",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.SharedGroup = ""
			},
			wantErr: "EMQX_SHARED_GROUP is required",
		},
		{
			name: "invalid QoS",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.QoS = 3
			},
			wantErr: "EMQX_QOS must be 0, 1, or 2",
		},
		{
			name: "invalid workers",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.Workers = 0
			},
			wantErr: "INGESTOR_WORKERS must be >= 1",
		},
		{
			name: "invalid queue size",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.QueueSize = 0
			},
			wantErr: "INGESTOR_QUEUE_SIZE must be >= 1",
		},
		{
			name: "invalid drop policy",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.DropPolicy = "invalid"
			},
			wantErr: "INGESTOR_DROP_POLICY must be 'drop_new' or 'coalesce_latest'",
		},
		{
			name: "negative max retries",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.NackMaxRetries = -1
			},
			wantErr: "NACK_MAX_RETRIES must be >= 0",
		},
		{
			name: "zero heartbeat TTL",
			modify: func(c *Config) {
				c.ClientID = "test-client"
				c.HeartbeatTTL = 0
			},
			wantErr: "HEARTBEAT_TTL must be > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoadConfig()
			tt.modify(&cfg)

			err := cfg.Validate()

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestConfigSharedSubscriptionTopic(t *testing.T) {
	cfg := Config{
		SharedGroup: "my-group",
		TopicFilter: "bus/+/gps",
	}

	topic := cfg.SharedSubscriptionTopic()

	assert.Equal(t, "$share/my-group/bus/+/gps", topic)
}

func TestConfigParsedServerURLs(t *testing.T) {
	cfg := Config{
		ServerURLs: []string{
			"mqtt://broker1:1883",
			"tls://broker2:8883",
		},
	}

	urls, err := cfg.ParsedServerURLs()

	require.NoError(t, err)
	require.Len(t, urls, 2)
	assert.Equal(t, "mqtt", urls[0].Scheme)
	assert.Equal(t, "broker1:1883", urls[0].Host)
	assert.Equal(t, "tls", urls[1].Scheme)
	assert.Equal(t, "broker2:8883", urls[1].Host)
}

func TestConfigParsedServerURLs_Invalid(t *testing.T) {
	cfg := Config{
		ServerURLs: []string{"://invalid"},
	}

	urls, err := cfg.ParsedServerURLs()

	assert.Error(t, err)
	assert.Nil(t, urls)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper function tests
// ─────────────────────────────────────────────────────────────────────────────

func TestParseStringSlice(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{"  a  ,  b  ,  c  ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseStringSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	os.Setenv("TEST_DURATION", "5s")
	defer os.Unsetenv("TEST_DURATION")

	d := getEnvDuration("TEST_DURATION", 1*time.Second)
	assert.Equal(t, 5*time.Second, d)

	// Test default when not set
	d = getEnvDuration("TEST_DURATION_MISSING", 10*time.Second)
	assert.Equal(t, 10*time.Second, d)

	// Test default when invalid
	os.Setenv("TEST_DURATION_INVALID", "not-a-duration")
	defer os.Unsetenv("TEST_DURATION_INVALID")
	d = getEnvDuration("TEST_DURATION_INVALID", 20*time.Second)
	assert.Equal(t, 20*time.Second, d)
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 0: GPS Enrichment Config Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestConfig_GPSEnrichmentEnabled_DefaultOff(t *testing.T) {
	// Clear the env var to test default
	os.Unsetenv("ENABLE_GPS_ENRICHMENT")

	cfg := LoadConfig()

	// CRITICAL: GPS enrichment MUST default to OFF (feature flag safety)
	assert.False(t, cfg.GPSEnrichmentEnabled, "GPS enrichment must default to false (OFF)")
}

func TestConfig_GPSEnrichmentEnabled_ExplicitOn(t *testing.T) {
	os.Setenv("ENABLE_GPS_ENRICHMENT", "true")
	defer os.Unsetenv("ENABLE_GPS_ENRICHMENT")

	cfg := LoadConfig()

	assert.True(t, cfg.GPSEnrichmentEnabled, "GPS enrichment should be enabled when explicitly set to true")
}

func TestConfig_GPSEnrichmentEnabled_ExplicitOff(t *testing.T) {
	os.Setenv("ENABLE_GPS_ENRICHMENT", "false")
	defer os.Unsetenv("ENABLE_GPS_ENRICHMENT")

	cfg := LoadConfig()

	assert.False(t, cfg.GPSEnrichmentEnabled, "GPS enrichment should be disabled when explicitly set to false")
}

func TestConfig_MultiTopicParsing(t *testing.T) {
	// Test that multi-topic filter produces correct subscriptions
	// This is used for subscribing to multiple topics (comma-separated)
	tests := []struct {
		name           string
		topicFilter    string
		expectedTopics []string
	}{
		{
			name:           "single topic (existing behavior)",
			topicFilter:    "bus/+/gps",
			expectedTopics: []string{"bus/+/gps"},
		},
		{
			name:           "multi-topic",
			topicFilter:    "bus/+/gps,bus/+/gps2",
			expectedTopics: []string{"bus/+/gps", "bus/+/gps2"},
		},
		{
			name:           "multi-topic with spaces",
			topicFilter:    "bus/+/gps, bus/+/gps2",
			expectedTopics: []string{"bus/+/gps", "bus/+/gps2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("EMQX_TOPIC_FILTER", tt.topicFilter)
			defer os.Unsetenv("EMQX_TOPIC_FILTER")

			cfg := LoadConfig()

			// Parse the topic filter using the same parseStringSlice logic
			topics := parseStringSlice(cfg.TopicFilter)
			assert.Equal(t, tt.expectedTopics, topics, "Multi-topic parsing should produce correct subscriptions")
		})
	}
}

func TestConfig_MultiTopicSharedSubscription(t *testing.T) {
	// Verify that shared subscription topic format is correct for multi-topic
	cfg := Config{
		SharedGroup: "alebus-ingestor",
		TopicFilter: "bus/+/gps,bus/+/gps2",
	}

	// Note: SharedSubscriptionTopic returns single topic string
	// For multi-topic, we'd need to split and create multiple subscriptions
	topic := cfg.SharedSubscriptionTopic()
	assert.Equal(t, "$share/alebus-ingestor/bus/+/gps,bus/+/gps2", topic)

	// The worker will need to parse this or subscribe to each topic separately
	// This test documents current behavior - actual multi-subscription handled in worker
}

// ═══════════════════════════════════════════════════════════════════════════════
// Phase 8: Staged Rollout Configuration Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestConfig_GPSAllowedBusPrefixes_FromEnv(t *testing.T) {
	tests := []struct {
		name             string
		envValue         string
		expectedPrefixes []string
	}{
		{
			name:             "empty (no filtering)",
			envValue:         "",
			expectedPrefixes: nil,
		},
		{
			name:             "single prefix",
			envValue:         "BUS-001",
			expectedPrefixes: []string{"BUS-001"},
		},
		{
			name:             "multiple prefixes",
			envValue:         "BUS-001,BUS-002,ROUTE-A-",
			expectedPrefixes: []string{"BUS-001", "BUS-002", "ROUTE-A-"},
		},
		{
			name:             "prefixes with spaces",
			envValue:         "BUS-001, BUS-002, ROUTE-A-",
			expectedPrefixes: []string{"BUS-001", "BUS-002", "ROUTE-A-"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("GPS_ALLOWED_BUS_PREFIXES", tt.envValue)
				defer os.Unsetenv("GPS_ALLOWED_BUS_PREFIXES")
			}

			cfg := LoadConfig()
			assert.Equal(t, tt.expectedPrefixes, cfg.GPSAllowedBusPrefixes)
		})
	}
}

func TestConfig_GPSRolloutPercentage_FromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{"default (no env)", "", 0},
		{"zero", "0", 0},
		{"ten percent", "10", 10},
		{"fifty percent", "50", 50},
		{"hundred percent", "100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("GPS_ROLLOUT_PERCENTAGE", tt.envValue)
				defer os.Unsetenv("GPS_ROLLOUT_PERCENTAGE")
			}

			cfg := LoadConfig()
			assert.Equal(t, tt.expected, cfg.GPSRolloutPercentage)
		})
	}
}

func TestConfig_GPSRolloutPercentage_Validation(t *testing.T) {
	tests := []struct {
		name       string
		percentage int
		shouldFail bool
	}{
		{"valid 0", 0, false},
		{"valid 50", 50, false},
		{"valid 100", 100, false},
		{"invalid -1", -1, true},
		{"invalid 101", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoadConfig()
			cfg.ClientID = "test-client"
			cfg.GPSRolloutPercentage = tt.percentage

			err := cfg.Validate()
			if tt.shouldFail {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "GPS_ROLLOUT_PERCENTAGE")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_IsBusAllowedForGPSEnrichment(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		prefixes   []string
		percentage int
		busID      string
		expected   bool
	}{
		// Feature flag OFF
		{"disabled - always false", false, nil, 0, "BUS-001", false},
		{"disabled with prefixes - still false", false, []string{"BUS-"}, 0, "BUS-001", false},

		// Feature flag ON, no filters
		{"enabled, no filters - all allowed", true, nil, 0, "BUS-001", true},
		{"enabled, no filters - any bus", true, nil, 0, "RANDOM-ID", true},

		// Prefix filtering
		{"prefix match - allowed", true, []string{"BUS-"}, 0, "BUS-001", true},
		{"prefix no match - blocked", true, []string{"BUS-"}, 0, "TRAIN-001", false},
		{"multi-prefix match first", true, []string{"BUS-", "TRAIN-"}, 0, "BUS-001", true},
		{"multi-prefix match second", true, []string{"BUS-", "TRAIN-"}, 0, "TRAIN-001", true},
		{"multi-prefix no match", true, []string{"BUS-", "TRAIN-"}, 0, "TRAM-001", false},

		// Percentage filtering (hash-based, deterministic)
		{"100% rollout - all allowed", true, nil, 100, "ANY-BUS", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				GPSEnrichmentEnabled:  tt.enabled,
				GPSAllowedBusPrefixes: tt.prefixes,
				GPSRolloutPercentage:  tt.percentage,
			}

			result := cfg.IsBusAllowedForGPSEnrichment(tt.busID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_IsBusAllowedForGPSEnrichment_PercentageDeterministic(t *testing.T) {
	// Verify that percentage rollout is deterministic (same bus always gets same result)
	cfg := Config{
		GPSEnrichmentEnabled: true,
		GPSRolloutPercentage: 50, // 50% of buses
	}

	// Test multiple times - should always get same result
	busID := "BUS-TEST-001"
	firstResult := cfg.IsBusAllowedForGPSEnrichment(busID)

	for i := 0; i < 100; i++ {
		result := cfg.IsBusAllowedForGPSEnrichment(busID)
		assert.Equal(t, firstResult, result, "Percentage rollout should be deterministic")
	}
}

func TestConfig_IsBusAllowedForGPSEnrichment_PercentageDistribution(t *testing.T) {
	// Verify that percentage roughly matches expected distribution
	cfg := Config{
		GPSEnrichmentEnabled: true,
		GPSRolloutPercentage: 10, // 10% of buses
	}

	allowed := 0
	total := 1000

	for i := 0; i < total; i++ {
		busID := fmt.Sprintf("BUS-%04d", i)
		if cfg.IsBusAllowedForGPSEnrichment(busID) {
			allowed++
		}
	}

	// With 10% rollout, expect ~100 buses allowed (allow 5% tolerance)
	expectedMin := int(float64(total) * 0.05) // 5%
	expectedMax := int(float64(total) * 0.15) // 15%

	assert.GreaterOrEqual(t, allowed, expectedMin, "Too few buses allowed for 10% rollout")
	assert.LessOrEqual(t, allowed, expectedMax, "Too many buses allowed for 10% rollout")

	t.Logf("10%% rollout: %d/%d buses allowed (%.1f%%)", allowed, total, float64(allowed)/float64(total)*100)
}

func TestHashBusID_Deterministic(t *testing.T) {
	// Verify the hash function is deterministic
	busID := "BUS-TEST-12345"
	firstHash := hashBusID(busID)

	for i := 0; i < 100; i++ {
		hash := hashBusID(busID)
		assert.Equal(t, firstHash, hash, "Hash should be deterministic")
	}

	// Verify hash is in valid range [0, 100)
	for i := 0; i < 1000; i++ {
		busID := fmt.Sprintf("BUS-%d", i)
		hash := hashBusID(busID)
		assert.GreaterOrEqual(t, hash, 0)
		assert.Less(t, hash, 100)
	}
}
