package mqtt

import (
	"fmt"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// GPS Message Parsing Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestParseGPSMessage_ValidMessage(t *testing.T) {
	payload := []byte(`{
		"bus_id": "BUS001",
		"lat": -33.8650,
		"lon": 151.2093,
		"timestamp_ms": 1704067200000,
		"speed_kmh": 45.5,
		"heading": 90.0,
		"accuracy_m": 5.0
	}`)

	msg, err := ParseGPSMessage(payload)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if msg.BusID != "BUS001" {
		t.Errorf("Expected BusID=BUS001, got %s", msg.BusID)
	}
	if msg.Lat != -33.8650 {
		t.Errorf("Expected Lat=-33.8650, got %f", msg.Lat)
	}
	if msg.Lon != 151.2093 {
		t.Errorf("Expected Lon=151.2093, got %f", msg.Lon)
	}
	if msg.TimestampMs != 1704067200000 {
		t.Errorf("Expected TimestampMs=1704067200000, got %d", msg.TimestampMs)
	}
	if msg.SpeedKmh != 45.5 {
		t.Errorf("Expected SpeedKmh=45.5, got %f", msg.SpeedKmh)
	}
	if msg.Heading != 90.0 {
		t.Errorf("Expected Heading=90.0, got %f", msg.Heading)
	}
	if msg.AccuracyM != 5.0 {
		t.Errorf("Expected AccuracyM=5.0, got %f", msg.AccuracyM)
	}
}

func TestParseGPSMessage_MinimalPayload(t *testing.T) {
	// Only required fields
	payload := []byte(`{
		"bus_id": "BUS001",
		"lat": -33.8650,
		"lon": 151.2093,
		"timestamp_ms": 1704067200000
	}`)

	msg, err := ParseGPSMessage(payload)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if msg.BusID != "BUS001" {
		t.Errorf("Expected BusID=BUS001, got %s", msg.BusID)
	}
	// Optional fields should be zero
	if msg.SpeedKmh != 0 {
		t.Errorf("Expected SpeedKmh=0, got %f", msg.SpeedKmh)
	}
	if msg.Heading != 0 {
		t.Errorf("Expected Heading=0, got %f", msg.Heading)
	}
	if msg.AccuracyM != 0 {
		t.Errorf("Expected AccuracyM=0, got %f", msg.AccuracyM)
	}
}

func TestParseGPSMessage_InvalidJSON(t *testing.T) {
	payload := []byte(`{invalid json`)

	_, err := ParseGPSMessage(payload)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestParseGPSMessage_EmptyPayload(t *testing.T) {
	payload := []byte(`{}`)

	msg, err := ParseGPSMessage(payload)
	if err != nil {
		t.Fatalf("ParseGPSMessage should succeed for empty object, validation catches it")
	}

	// Validate should fail
	if err := msg.Validate(); err == nil {
		t.Error("Expected validation to fail for empty object")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// GPS Message Validation Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestRawGPSMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		msg     RawGPSMessage
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid message passes",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				SpeedKmh:    45.5,
				Heading:     90.0,
				AccuracyM:   5.0,
			},
			wantErr: false,
		},
		{
			name: "Empty BusID fails",
			msg: RawGPSMessage{
				BusID:       "",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
			},
			wantErr: true,
			errMsg:  "bus_id",
		},
		{
			name: "Lat below -90 fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -91.0,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
			},
			wantErr: true,
			errMsg:  "lat",
		},
		{
			name: "Lat above 90 fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         91.0,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
			},
			wantErr: true,
			errMsg:  "lat",
		},
		{
			name: "Lon below -180 fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         -181.0,
				TimestampMs: 1704067200000,
			},
			wantErr: true,
			errMsg:  "lon",
		},
		{
			name: "Lon above 180 fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         181.0,
				TimestampMs: 1704067200000,
			},
			wantErr: true,
			errMsg:  "lon",
		},
		{
			name: "Zero TimestampMs fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 0,
			},
			wantErr: true,
			errMsg:  "timestamp_ms",
		},
		{
			name: "Negative TimestampMs fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: -1,
			},
			wantErr: true,
			errMsg:  "timestamp_ms",
		},
		{
			name: "Negative SpeedKmh fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				SpeedKmh:    -1.0,
			},
			wantErr: true,
			errMsg:  "speed_kmh",
		},
		{
			name: "Heading below 0 fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				Heading:     -1.0,
			},
			wantErr: true,
			errMsg:  "heading",
		},
		{
			name: "Heading above 360 fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				Heading:     361.0,
			},
			wantErr: true,
			errMsg:  "heading",
		},
		{
			name: "Negative AccuracyM fails",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				AccuracyM:   -1.0,
			},
			wantErr: true,
			errMsg:  "accuracy_m",
		},
		{
			name: "Boundary Lat -90 passes",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -90.0,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
			},
			wantErr: false,
		},
		{
			name: "Boundary Lat 90 passes",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         90.0,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
			},
			wantErr: false,
		},
		{
			name: "Boundary Lon -180 passes",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         -180.0,
				TimestampMs: 1704067200000,
			},
			wantErr: false,
		},
		{
			name: "Boundary Lon 180 passes",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         180.0,
				TimestampMs: 1704067200000,
			},
			wantErr: false,
		},
		{
			name: "Boundary Heading 0 passes",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				Heading:     0.0,
			},
			wantErr: false,
		},
		{
			name: "Boundary Heading 360 passes",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				Heading:     360.0,
			},
			wantErr: false,
		},
		{
			name: "Zero SpeedKmh passes (optional)",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				SpeedKmh:    0.0,
			},
			wantErr: false,
		},
		{
			name: "Zero AccuracyM passes (optional)",
			msg: RawGPSMessage{
				BusID:       "BUS001",
				Lat:         -33.8650,
				Lon:         151.2093,
				TimestampMs: 1704067200000,
				AccuracyM:   0.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errMsg != "" {
					// Check that error mentions the expected field
					if !containsString(err.Error(), tt.errMsg) {
						t.Errorf("Expected error to mention '%s', got: %s", tt.errMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ToRawGPSUpdate Conversion Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestRawGPSMessage_ToRawGPSUpdate(t *testing.T) {
	msg := RawGPSMessage{
		BusID:       "BUS001",
		Lat:         -33.8650,
		Lon:         151.2093,
		TimestampMs: 1704067200000,
		SpeedKmh:    45.5,
		Heading:     90.0,
		AccuracyM:   5.0,
	}

	update := msg.ToRawGPSUpdate()

	if update.BusID != msg.BusID {
		t.Errorf("Expected BusID=%s, got %s", msg.BusID, update.BusID)
	}
	if update.Lat != msg.Lat {
		t.Errorf("Expected Lat=%f, got %f", msg.Lat, update.Lat)
	}
	if update.Lon != msg.Lon {
		t.Errorf("Expected Lon=%f, got %f", msg.Lon, update.Lon)
	}
	if update.Timestamp.UnixMilli() != msg.TimestampMs {
		t.Errorf("Expected Timestamp=%d ms, got %d ms", msg.TimestampMs, update.Timestamp.UnixMilli())
	}
	if update.SpeedKmh != msg.SpeedKmh {
		t.Errorf("Expected SpeedKmh=%f, got %f", msg.SpeedKmh, update.SpeedKmh)
	}
	if update.Heading != msg.Heading {
		t.Errorf("Expected Heading=%f, got %f", msg.Heading, update.Heading)
	}
	if update.AccuracyM != msg.AccuracyM {
		t.Errorf("Expected AccuracyM=%f, got %f", msg.AccuracyM, update.AccuracyM)
	}
}

func TestRawGPSMessage_ToRawGPSUpdate_TimestampConversion(t *testing.T) {
	// Use a known timestamp
	knownTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	msg := RawGPSMessage{
		BusID:       "BUS001",
		Lat:         -33.8650,
		Lon:         151.2093,
		TimestampMs: knownTime.UnixMilli(),
	}

	update := msg.ToRawGPSUpdate()

	if !update.Timestamp.Equal(knownTime) {
		t.Errorf("Expected Timestamp=%v, got %v", knownTime, update.Timestamp)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Topic Classification Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestClassifyTopic(t *testing.T) {
	tests := []struct {
		topic    string
		expected TopicType
	}{
		{"bus/BUS001/telemetry", TopicTypeUnknown},
		{"bus/BUS002/telemetry", TopicTypeUnknown},
		{"bus/ABC123/telemetry", TopicTypeUnknown},
		{"bus/BUS001/gps", TopicTypeGPS},
		{"bus/BUS002/gps", TopicTypeGPS},
		{"bus/ABC123/gps", TopicTypeGPS},
		{"bus/BUS001/status", TopicTypeUnknown},
		{"other/topic", TopicTypeUnknown},
		{"", TopicTypeUnknown},
		{"telemetry", TopicTypeUnknown},
		{"gps", TopicTypeUnknown},
		{"/telemetry", TopicTypeUnknown},
		{"/gps", TopicTypeGPS},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			result := ClassifyTopic(tt.topic)
			if result != tt.expected {
				t.Errorf("ClassifyTopic(%q) = %v, want %v", tt.topic, result, tt.expected)
			}
		})
	}
}

func TestExtractBusIDFromTopic(t *testing.T) {
	tests := []struct {
		topic    string
		expected string
	}{
		{"bus/BUS001/telemetry", "BUS001"},
		{"bus/BUS002/gps", "BUS002"},
		{"bus/ABC123/status", "ABC123"},
		{"other/topic", ""},
		{"bus", ""},
		{"", ""},
		{"/gps", ""},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			result := ExtractBusIDFromTopic(tt.topic)
			if result != tt.expected {
				t.Errorf("ExtractBusIDFromTopic(%q) = %q, want %q", tt.topic, result, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Validation Error Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestGPSMessageValidationError_Error(t *testing.T) {
	err := &GPSMessageValidationError{
		Field:  "lat",
		Reason: "must be in range [-90, 90]",
	}

	expected := "invalid lat: must be in range [-90, 90]"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestIsGPSMessageValidationError(t *testing.T) {
	t.Run("GPSMessageValidationError returns true", func(t *testing.T) {
		err := &GPSMessageValidationError{Field: "lat", Reason: "test"}
		if !IsGPSMessageValidationError(err) {
			t.Error("Expected true for GPSMessageValidationError")
		}
	})

	t.Run("Other error returns false", func(t *testing.T) {
		err := fmt.Errorf("some other error")
		if IsGPSMessageValidationError(err) {
			t.Error("Expected false for regular error")
		}
	})

	t.Run("Nil returns false", func(t *testing.T) {
		if IsGPSMessageValidationError(nil) {
			t.Error("Expected false for nil")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Topic Type String Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestTopicType_String(t *testing.T) {
	tests := []struct {
		topicType TopicType
		expected  string
	}{
		{TopicTypeGPS, "gps"},
		{TopicTypeUnknown, "unknown"},
		{TopicType(99), "TopicType(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.topicType.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.topicType.String())
			}
		})
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
