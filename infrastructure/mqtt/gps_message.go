package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MathewM27/busTrack-alebus/application/journey/ports"
)

// ─────────────────────────────────────────────────────────────────────────────
// GPS Message Types (Phase 5 - Option B+)
// ─────────────────────────────────────────────────────────────────────────────
//
// This file defines the wire format for RAW GPS telemetry messages.
// These messages come from bus/+/gps topics and contain ONLY GPS data:
//   - bus_id, lat, lon, timestamp_ms, speed_kmh, heading, accuracy_m
//   - NO route_id, direction, stop_index (enriched by application layer)
//
// This repo is now GPS-only: pre-enriched telemetry ingestion has been removed.
//
// ─────────────────────────────────────────────────────────────────────────────

// RawGPSMessage is the JSON structure for raw GPS telemetry from MQTT.
// This is the wire format from real bus GPS devices.
//
// Key characteristics:
//   - NO route_id (looked up from bus assignment)
//   - NO direction (inferred by resolver)
//   - NO stop_index (computed by resolver)
//   - NO is_at_terminal (detected by resolver)
type RawGPSMessage struct {
	// BusID is the unique bus identifier. Required.
	BusID string `json:"bus_id"`

	// Lat is the GPS latitude. Required, must be in [-90, 90].
	Lat float64 `json:"lat"`

	// Lon is the GPS longitude. Required, must be in [-180, 180].
	Lon float64 `json:"lon"`

	// TimestampMs is Unix milliseconds (device timestamp). Required.
	TimestampMs int64 `json:"timestamp_ms"`

	// SpeedKmh is the current speed in km/h. Optional, defaults to 0.
	SpeedKmh float64 `json:"speed_kmh,omitempty"`

	// Heading is the compass heading in degrees [0, 360]. Optional, defaults to 0.
	Heading float64 `json:"heading,omitempty"`

	// AccuracyM is the GPS accuracy radius in meters. Optional, defaults to 0.
	AccuracyM float64 `json:"accuracy_m,omitempty"`
}

// Validate performs validation on the raw GPS message.
// Returns an error if required fields are missing or invalid.
//
// Validation rules:
//   - bus_id: required, non-empty
//   - lat: required, must be in [-90, 90]
//   - lon: required, must be in [-180, 180]
//   - timestamp_ms: required, must be > 0
//   - speed_kmh: if present, must be >= 0
//   - heading: if present, must be in [0, 360]
//   - accuracy_m: if present, must be >= 0
func (m *RawGPSMessage) Validate() error {
	if m.BusID == "" {
		return &GPSMessageValidationError{Field: "bus_id", Reason: "required"}
	}
	if m.Lat < -90 || m.Lat > 90 {
		return &GPSMessageValidationError{Field: "lat", Reason: "must be in range [-90, 90]"}
	}
	if m.Lon < -180 || m.Lon > 180 {
		return &GPSMessageValidationError{Field: "lon", Reason: "must be in range [-180, 180]"}
	}
	if m.TimestampMs <= 0 {
		return &GPSMessageValidationError{Field: "timestamp_ms", Reason: "required and must be > 0"}
	}
	if m.SpeedKmh < 0 {
		return &GPSMessageValidationError{Field: "speed_kmh", Reason: "must be >= 0"}
	}
	if m.Heading < 0 || m.Heading > 360 {
		return &GPSMessageValidationError{Field: "heading", Reason: "must be in range [0, 360]"}
	}
	if m.AccuracyM < 0 {
		return &GPSMessageValidationError{Field: "accuracy_m", Reason: "must be >= 0"}
	}
	return nil
}

// ToRawGPSUpdate converts the wire message to the application-layer DTO.
// This DTO is consumed by GPSEnrichmentUseCase.
func (m *RawGPSMessage) ToRawGPSUpdate() ports.RawGPSUpdate {
	return ports.RawGPSUpdate{
		BusID:     m.BusID,
		Lat:       m.Lat,
		Lon:       m.Lon,
		Timestamp: time.UnixMilli(m.TimestampMs),
		SpeedKmh:  m.SpeedKmh,
		Heading:   m.Heading,
		AccuracyM: m.AccuracyM,
	}
}

// ParseGPSMessage parses JSON bytes into a RawGPSMessage.
func ParseGPSMessage(data []byte) (*RawGPSMessage, error) {
	var msg RawGPSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse GPS JSON: %w", err)
	}
	return &msg, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Validation Error
// ─────────────────────────────────────────────────────────────────────────────

// GPSMessageValidationError represents a validation failure for GPS messages.
type GPSMessageValidationError struct {
	Field  string
	Reason string
}

func (e *GPSMessageValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// IsGPSMessageValidationError checks if an error is a GPS validation error.
func IsGPSMessageValidationError(err error) bool {
	_, ok := err.(*GPSMessageValidationError)
	return ok
}

// ─────────────────────────────────────────────────────────────────────────────
// Topic Routing
// ─────────────────────────────────────────────────────────────────────────────

// TopicType identifies the type of MQTT topic.
type TopicType int

const (
	// TopicTypeGPS is for raw GPS data (bus/+/gps).
	TopicTypeGPS TopicType = iota

	// TopicTypeUnknown is for unrecognized topic patterns.
	TopicTypeUnknown
)

func (t TopicType) String() string {
	switch t {
	case TopicTypeGPS:
		return "gps"
	case TopicTypeUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("TopicType(%d)", t)
	}
}

// ClassifyTopic determines the topic type based on the topic string.
// Supports patterns like:
//   - bus/BUS001/gps → TopicTypeGPS
//   - Other patterns → TopicTypeUnknown
func ClassifyTopic(topic string) TopicType {
	if strings.HasSuffix(topic, "/gps") {
		return TopicTypeGPS
	}
	return TopicTypeUnknown
}

// ExtractBusIDFromTopic extracts the bus ID from a topic like "bus/{busID}/...".
// Returns empty string if the topic doesn't match the expected pattern.
func ExtractBusIDFromTopic(topic string) string {
	// Expected format: bus/{busID}/gps
	parts := strings.Split(topic, "/")
	if len(parts) >= 2 && parts[0] == "bus" {
		return parts[1]
	}
	return ""
}
