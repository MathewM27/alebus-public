package redis

// LiveBusUpdatesChannel is the Redis PubSub channel that emits a message
// whenever a live bus state update is successfully applied.
//
// Payload: busId (string)
//
// Notes:
// - PubSub is fire-and-forget (no replay). Consumers MUST be able to re-fetch
//   the latest state from Redis upon reconnect.
// - We publish from within the atomic Lua script to preserve the "single EVAL"
//   requirement of ports.LiveBusPublisher.
const LiveBusUpdatesChannel = "alebus:livebus:updates"

// JourneyUpdatesChannel is the Redis PubSub channel that emits journey tracking
// updates computed by the background worker.
//
// Payload: JSON (see application/journey/streaming.JourneyUpdate)
//
// Notes:
// - PubSub is fire-and-forget (no replay). Consumers should treat it as a live
//   notification channel, not a durable event log.
const JourneyUpdatesChannel = "alebus:journey:updates"
