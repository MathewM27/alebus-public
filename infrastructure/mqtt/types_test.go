package mqtt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ─────────────────────────────────────────────────────────────────────────────
// AckDecision Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckDecision_String(t *testing.T) {
	assert.Equal(t, "ACK", AckDecisionACK.String())
	assert.Equal(t, "RETRY", AckDecisionRetry.String())
	assert.Equal(t, "AckDecision(99)", AckDecision(99).String())
}

func TestMessageOutcome_String(t *testing.T) {
	assert.Equal(t, "accepted", OutcomeAccepted.String())
	assert.Equal(t, "stale", OutcomeStale.String())
	assert.Equal(t, "invalid", OutcomeInvalid.String())
	assert.Equal(t, "infra_error", OutcomeInfraError.String())
	assert.Equal(t, "dropped", OutcomeDropped.String())
	assert.Equal(t, "coalesced", OutcomeCoalesced.String())
	assert.Equal(t, "gps_invalid", OutcomeGPSInvalid.String())
	assert.Equal(t, "gps_success", OutcomeGPSSuccess.String())
	assert.Equal(t, "MessageOutcome(99)", MessageOutcome(99).String())
}

// ─────────────────────────────────────────────────────────────────────────────
// WorkItem Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNewWorkItem(t *testing.T) {
	before := time.Now()
	item := NewWorkItem(42, "bus-123", []byte(`{"test":true}`))
	after := time.Now()

	assert.Equal(t, uint64(42), item.RecvSeq)
	assert.Equal(t, "bus-123", item.BusID)
	assert.Equal(t, []byte(`{"test":true}`), item.RawPayload)
	assert.True(t, item.ReceivedAt.After(before) || item.ReceivedAt.Equal(before))
	assert.True(t, item.ReceivedAt.Before(after) || item.ReceivedAt.Equal(after))
	assert.NotNil(t, item.ResultCh)
	assert.Equal(t, 0, item.RetryCount)
}
