package ports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// RawGPSUpdate Validation Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestRawGPSUpdate_Validate(t *testing.T) {
	validUpdate := func() RawGPSUpdate {
		return RawGPSUpdate{
			BusID:     "BUS001",
			Lat:       -33.8688,
			Lon:       151.2093,
			Timestamp: time.Now(),
			SpeedKmh:  35.5,
			Heading:   180.0,
			AccuracyM: 5.0,
		}
	}

	tests := []struct {
		name    string
		modify  func(*RawGPSUpdate)
		wantErr string
	}{
		{
			name:    "Valid update passes",
			modify:  func(u *RawGPSUpdate) {},
			wantErr: "",
		},
		{
			name: "Empty BusID fails",
			modify: func(u *RawGPSUpdate) {
				u.BusID = ""
			},
			wantErr: "BusID required",
		},
		{
			name: "Lat below -90 fails",
			modify: func(u *RawGPSUpdate) {
				u.Lat = -91.0
			},
			wantErr: "Lat must be in range",
		},
		{
			name: "Lat above 90 fails",
			modify: func(u *RawGPSUpdate) {
				u.Lat = 91.0
			},
			wantErr: "Lat must be in range",
		},
		{
			name: "Lon below -180 fails",
			modify: func(u *RawGPSUpdate) {
				u.Lon = -181.0
			},
			wantErr: "Lon must be in range",
		},
		{
			name: "Lon above 180 fails",
			modify: func(u *RawGPSUpdate) {
				u.Lon = 181.0
			},
			wantErr: "Lon must be in range",
		},
		{
			name: "Zero Timestamp fails",
			modify: func(u *RawGPSUpdate) {
				u.Timestamp = time.Time{}
			},
			wantErr: "Timestamp required",
		},
		{
			name: "Negative SpeedKmh fails",
			modify: func(u *RawGPSUpdate) {
				u.SpeedKmh = -1.0
			},
			wantErr: "SpeedKmh must be >= 0",
		},
		{
			name: "Heading below 0 fails",
			modify: func(u *RawGPSUpdate) {
				u.Heading = -1.0
			},
			wantErr: "Heading must be in range",
		},
		{
			name: "Heading above 360 fails",
			modify: func(u *RawGPSUpdate) {
				u.Heading = 361.0
			},
			wantErr: "Heading must be in range",
		},
		{
			name: "Negative AccuracyM fails",
			modify: func(u *RawGPSUpdate) {
				u.AccuracyM = -1.0
			},
			wantErr: "AccuracyM must be >= 0",
		},
		{
			name: "Boundary Lat -90 passes",
			modify: func(u *RawGPSUpdate) {
				u.Lat = -90.0
			},
			wantErr: "",
		},
		{
			name: "Boundary Lat 90 passes",
			modify: func(u *RawGPSUpdate) {
				u.Lat = 90.0
			},
			wantErr: "",
		},
		{
			name: "Boundary Lon -180 passes",
			modify: func(u *RawGPSUpdate) {
				u.Lon = -180.0
			},
			wantErr: "",
		},
		{
			name: "Boundary Lon 180 passes",
			modify: func(u *RawGPSUpdate) {
				u.Lon = 180.0
			},
			wantErr: "",
		},
		{
			name: "Boundary Heading 0 passes",
			modify: func(u *RawGPSUpdate) {
				u.Heading = 0.0
			},
			wantErr: "",
		},
		{
			name: "Boundary Heading 360 passes",
			modify: func(u *RawGPSUpdate) {
				u.Heading = 360.0
			},
			wantErr: "",
		},
		{
			name: "Zero SpeedKmh passes (optional)",
			modify: func(u *RawGPSUpdate) {
				u.SpeedKmh = 0.0
			},
			wantErr: "",
		},
		{
			name: "Zero AccuracyM passes (optional)",
			modify: func(u *RawGPSUpdate) {
				u.AccuracyM = 0.0
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := validUpdate()
			tt.modify(&update)

			err := update.Validate()

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGPSValidationError_Error(t *testing.T) {
	err := &GPSValidationError{Field: "Lat", Reason: "must be in range [-90, 90]"}
	assert.Equal(t, "invalid GPS: Lat must be in range [-90, 90]", err.Error())
}

func TestIsGPSValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "GPSValidationError returns true",
			err:      &GPSValidationError{Field: "Lat", Reason: "invalid"},
			expected: true,
		},
		{
			name:     "Other error returns false",
			err:      assert.AnError,
			expected: false,
		},
		{
			name:     "Nil returns false",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGPSValidationError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableEnrichmentError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error is not retryable",
			err:      nil,
			expected: false,
		},
		{
			name:     "GPSValidationError is not retryable",
			err:      &GPSValidationError{Field: "Lat", Reason: "invalid"},
			expected: false,
		},
		{
			name:     "Other error is retryable (infra error)",
			err:      assert.AnError,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableEnrichmentError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Resolver Mode Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestResolverMode_Constants(t *testing.T) {
	// Verify mode constants match expected string values
	assert.Equal(t, ResolverMode("BOOTSTRAP"), ResolverModeBootstrap)
	assert.Equal(t, ResolverMode("TRACKING"), ResolverModeTracking)
	assert.Equal(t, ResolverMode("REACQUIRE"), ResolverModeReacquire)
}

// ─────────────────────────────────────────────────────────────────────────────
// Enrichment Reason Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestEnrichmentReason_Constants(t *testing.T) {
	// Verify reason constants for consistent error classification
	assert.Equal(t, "", EnrichmentReasonSuccess)
	assert.Equal(t, "invalid_gps", EnrichmentReasonInvalidGPS)
	assert.Equal(t, "no_assignment", EnrichmentReasonNoAssignment)
	assert.Equal(t, "route_not_found", EnrichmentReasonRouteNotFound)
	assert.Equal(t, "gps_enrichment_disabled", EnrichmentReasonDisabled)
}
