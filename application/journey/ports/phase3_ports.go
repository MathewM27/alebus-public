// Package ports defines infrastructure interfaces for journey-related operations.
// This file contains Phase 3 ports for Redis resilience and observability.
package ports

import (
	"context"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3: Active Bus Tracking Port
// ─────────────────────────────────────────────────────────────────────────────

// ActiveBusTracker tracks which buses are currently on active journeys.
//
// NOTE: This port originated in an earlier phase where write-failure behavior
// was described in HTTP terms. Current architecture keeps ports transport-agnostic:
// Redis write failures surface as errors, and the outer transport layer decides
// how to respond.
type ActiveBusTracker interface {
	// MarkActive marks a bus as active (on a journey).
	// Called when a journey starts.
	MarkActive(ctx context.Context, busID, journeyID string) error

	// MarkInactive marks a bus as inactive (journey ended).
	// Called when a journey ends or is cancelled.
	MarkInactive(ctx context.Context, busID, journeyID string) error

	// IsActive checks if a bus is currently active (on a journey).
	// Used by GPS handlers to determine failure response behavior.
	IsActive(ctx context.Context, busID string) (bool, error)

	// GetActiveBuses returns all currently active bus IDs.
	GetActiveBuses(ctx context.Context) ([]string, error)

	// GetActiveBusCount returns the number of active buses.
	GetActiveBusCount(ctx context.Context) (int64, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3: Redis Metrics Port
// ─────────────────────────────────────────────────────────────────────────────

// RedisMetricsSnapshot holds observable metrics for Redis operations.
type RedisMetricsSnapshot struct {
	// Write metrics
	WriteCount      int64
	WriteErrors     int64
	WriteLatencyP50 int64 // microseconds
	WriteLatencyP95 int64 // microseconds
	WriteLatencyP99 int64 // microseconds

	// Read metrics
	ReadCount      int64
	ReadErrors     int64
	ReadLatencyP50 int64 // microseconds
	ReadLatencyP95 int64 // microseconds
	ReadLatencyP99 int64 // microseconds

	// Memory metrics
	UsedMemoryBytes int64
	MaxMemoryBytes  int64
	MemoryPercent   float64

	// Bus status metrics
	ActiveBusCount  int64
	OfflineBusCount int64
	TotalBusCount   int64

	// Replication metrics
	ConnectedReplicas int64

	// AOF metrics
	AOFEnabled bool

	// Timestamp
	CollectedAt time.Time
}

// RedisMetricsCollector collects Redis metrics.
type RedisMetricsCollector interface {
	// Collect gathers all Redis metrics.
	Collect(ctx context.Context) (*RedisMetricsSnapshot, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3: Redis Health Check Port
// ─────────────────────────────────────────────────────────────────────────────

// RedisHealthStatus represents the health state of Redis.
type RedisHealthStatus string

const (
	RedisHealthy   RedisHealthStatus = "healthy"
	RedisDegraded  RedisHealthStatus = "degraded"
	RedisUnhealthy RedisHealthStatus = "unhealthy"
)

// RedisHealthCheck represents a Redis health check result.
type RedisHealthCheck struct {
	Status         RedisHealthStatus
	Latency        time.Duration
	MemoryPercent  float64
	ConnectedNodes int
	Message        string
	CheckedAt      time.Time
}

// RedisHealthChecker performs Redis health checks.
type RedisHealthChecker interface {
	// Check performs a health check.
	Check(ctx context.Context) *RedisHealthCheck
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3: Failure Behavior Port
// ─────────────────────────────────────────────────────────────────────────────

// WriteFailureBehavior determines how to handle write failures.
type WriteFailureBehavior int

const (
	// WriteFailureStrict indicates the caller should treat a write failure as a hard failure.
	WriteFailureStrict WriteFailureBehavior = iota

	// WriteFailureGraceful indicates the caller may treat a write failure as a soft failure.
	WriteFailureGraceful
)

// FailureBehaviorDecider determines how to handle write failures for a bus.
type FailureBehaviorDecider interface {
	// DetermineFailureBehavior returns how to handle a write failure for the given bus.
	DetermineFailureBehavior(ctx context.Context, busID string) WriteFailureBehavior
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3: Latency Tracking Port
// ─────────────────────────────────────────────────────────────────────────────

// LatencyRecorder records operation latencies for observability.
type LatencyRecorder interface {
	// RecordWrite records a write operation's latency.
	RecordWrite(latency time.Duration)

	// RecordWriteError records a write error.
	RecordWriteError(err error)

	// RecordRead records a read operation's latency.
	RecordRead(latency time.Duration)

	// RecordReadError records a read error.
	RecordReadError(err error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3: Active Bus Cleanup Port
// ─────────────────────────────────────────────────────────────────────────────

// ActiveBusCleanupResult holds the result of a cleanup operation.
type ActiveBusCleanupResult struct {
	ScannedCount int64
	RemovedCount int64
	Duration     time.Duration
	Errors       []string
}

// ActiveBusCleaner cleans up stale entries from the active buses set.
type ActiveBusCleaner interface {
	// Cleanup removes stale entries from the active buses set.
	// A bus is considered stale if its last GPS update is older than the threshold.
	Cleanup(ctx context.Context) (*ActiveBusCleanupResult, error)
}
