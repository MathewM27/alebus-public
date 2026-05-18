package mqtt

import (
	"fmt"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// ACK Decision Types
// ─────────────────────────────────────────────────────────────────────────────

// AckDecision represents the acknowledgement decision for a received message.
// Per ADR-001, we implement deterministic ACK/NACK semantics.
type AckDecision int

const (
	// AckDecisionACK indicates the message should be acknowledged (PUBACK sent).
	// Used for: successful write, stale update, invalid payload, dropped/coalesced.
	AckDecisionACK AckDecision = iota

	// AckDecisionRetry indicates the message should be retried (withhold PUBACK).
	// Used for: infrastructure errors (Redis down/timeout) within retry budget.
	AckDecisionRetry
)

func (d AckDecision) String() string {
	switch d {
	case AckDecisionACK:
		return "ACK"
	case AckDecisionRetry:
		return "RETRY"
	default:
		return fmt.Sprintf("AckDecision(%d)", d)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Message Outcome Types (for metrics categorization)
// ─────────────────────────────────────────────────────────────────────────────

// MessageOutcome categorizes the result of processing a message.
// Used for metrics emission.
type MessageOutcome int

const (
	// OutcomeAccepted indicates successful Redis write.
	OutcomeAccepted MessageOutcome = iota

	// OutcomeStale indicates the update was rejected as stale.
	OutcomeStale

	// OutcomeInvalid indicates the payload failed validation.
	OutcomeInvalid

	// OutcomeInfraError indicates an infrastructure error (Redis down).
	OutcomeInfraError

	// OutcomeDropped indicates the message was dropped due to queue overflow.
	OutcomeDropped

	// OutcomeCoalesced indicates the message was coalesced (overwritten by newer).
	OutcomeCoalesced

	// ─────────────────────────────────────────────────────────────────────────
	// GPS Enrichment Outcomes (Phase 5 - Option B+)
	// ─────────────────────────────────────────────────────────────────────────

	// OutcomeGPSEnrichmentDisabled indicates GPS message was rejected because
	// the GPS enrichment feature flag is disabled.
	OutcomeGPSEnrichmentDisabled

	// OutcomeGPSStagedRolloutFiltered indicates GPS message was filtered out
	// by staged rollout configuration (prefix or percentage filtering).
	// Phase 8 - Gradual Rollout support.
	OutcomeGPSStagedRolloutFiltered

	// OutcomeGPSNoAssignment indicates no bus-to-route assignment was found.
	// This is a business rejection (ACK), not an infra error.
	OutcomeGPSNoAssignment

	// OutcomeGPSRouteNotFound indicates the route geometry was not found.
	// This is a business rejection (ACK), not an infra error.
	OutcomeGPSRouteNotFound

	// OutcomeGPSInvalid indicates the GPS payload failed validation.
	OutcomeGPSInvalid

	// OutcomeGPSReplayIgnored indicates the GPS payload was treated as a replay
	// at the edge due to device timestamp being too far in the past.
	OutcomeGPSReplayIgnored

	// OutcomeGPSFutureHardRejected indicates the GPS payload was rejected at the
	// edge because the device timestamp was egregiously in the future.
	OutcomeGPSFutureHardRejected

	// OutcomeGPSSuccess indicates successful GPS enrichment and Redis write.
	OutcomeGPSSuccess

	// OutcomeUnknownTopic indicates an unrecognized topic pattern.
	OutcomeUnknownTopic
)

func (o MessageOutcome) String() string {
	switch o {
	case OutcomeAccepted:
		return "accepted"
	case OutcomeStale:
		return "stale"
	case OutcomeInvalid:
		return "invalid"
	case OutcomeInfraError:
		return "infra_error"
	case OutcomeDropped:
		return "dropped"
	case OutcomeCoalesced:
		return "coalesced"
	case OutcomeGPSEnrichmentDisabled:
		return "gps_disabled"
	case OutcomeGPSStagedRolloutFiltered:
		return "gps_staged_rollout_filtered"
	case OutcomeGPSNoAssignment:
		return "gps_no_assignment"
	case OutcomeGPSRouteNotFound:
		return "gps_route_not_found"
	case OutcomeGPSInvalid:
		return "gps_invalid"
	case OutcomeGPSReplayIgnored:
		return "gps_replay_ignored"
	case OutcomeGPSFutureHardRejected:
		return "gps_future_hard_rejected"
	case OutcomeGPSSuccess:
		return "gps_success"
	case OutcomeUnknownTopic:
		return "unknown_topic"
	default:
		return fmt.Sprintf("MessageOutcome(%d)", o)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue Work Item
// ─────────────────────────────────────────────────────────────────────────────

// WorkItem represents a message queued for processing by a worker.
type WorkItem struct {
	// RecvSeq is the monotonically increasing sequence number (per-connection).
	// Used by ACK manager to enforce ordered acknowledgements.
	RecvSeq uint64

	// BusID is extracted from the message for coalescing lookups.
	BusID string

	// Topic is the MQTT topic the message was received on.
	// Used for routing to the appropriate processing path.
	Topic string

	// RawPayload is the original MQTT message payload (JSON bytes).
	RawPayload []byte

	// ReceivedAt is when the message was received from MQTT.
	ReceivedAt time.Time

	// ResultCh is where the worker reports the final AckDecision.
	// The ACK manager reads from this channel.
	ResultCh chan AckDecision

	// RetryCount tracks how many times this item has been retried.
	RetryCount int
}

// NewWorkItem creates a new work item for queue insertion.
func NewWorkItem(recvSeq uint64, busID string, payload []byte) *WorkItem {
	return &WorkItem{
		RecvSeq:    recvSeq,
		BusID:      busID,
		RawPayload: payload,
		ReceivedAt: time.Now(),
		ResultCh:   make(chan AckDecision, 1),
		RetryCount: 0,
	}
}

// NewWorkItemWithTopic creates a new work item with topic information.
// This is the preferred constructor for Phase 5+ when topic routing is needed.
func NewWorkItemWithTopic(recvSeq uint64, busID, topic string, payload []byte) *WorkItem {
	return &WorkItem{
		RecvSeq:    recvSeq,
		BusID:      busID,
		Topic:      topic,
		RawPayload: payload,
		ReceivedAt: time.Now(),
		ResultCh:   make(chan AckDecision, 1),
		RetryCount: 0,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Processing Result
// ─────────────────────────────────────────────────────────────────────────────

// ProcessingResult captures the outcome of processing a single work item.
type ProcessingResult struct {
	// Outcome categorizes the result for metrics.
	Outcome MessageOutcome

	// Decision is the ACK decision to send to the ACK manager.
	Decision AckDecision

	// Error is set if an infrastructure error occurred.
	Error error

	// Latency is how long the Redis EVAL took.
	Latency time.Duration
}
