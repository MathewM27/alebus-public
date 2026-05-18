# Observability Debug Workflow

## Overview

Three observability endpoints provide complete pipeline visibility from raw GPS → enrichment → Redis → recommendation ranking.

## Endpoints

### 1. Ingestor Last MQTT Event
**Purpose:** Shows the last raw MQTT message received and enrichment computation.

```bash
curl "http://127.0.0.1:9100/dev/obs/bus?busId=GPS-FPL-01-B"
```

**Response:**
```json
{
  "found": true,
  "bus_id": "GPS-FPL-01-B",
  "event": {
    "bus_id": "GPS-FPL-01-B",
    "topic": "bus/GPS-FPL-01-B/gps",
    "received_at": "2026-02-09T16:09:03.139302479Z",
    "raw_lat": -20.245,
    "raw_lon": 57.485,
    "raw_speed": 40,
    "raw_device_ts_ms": 1770653348135,
    "enriched": false,
    "interpolated": false,
    "is_at_terminal": false,
    "is_terminal_index": false,
    "distance_from_stop_m": 0
  }
}
```

**Use Case:**
- Verify MQTT messages are being received
- Check raw GPS coordinates before enrichment
- Debug enrichment logic (distance calculations, terminal detection)

---

### 2. Redis Bus State Snapshot
**Purpose:** Shows the final enriched state stored in Redis (what downstream consumers see).

```bash
curl -H "Authorization: Bearer devtoken" \
     -H "X-Dev-Secret: dev" \
     "http://127.0.0.1:9090/api/v1/dev/obs/redis/bus?busId=GPS-FPL-01-B"
```

**Response:**
```json
{
  "bus_id": "GPS-FPL-01-B",
  "redis_key": "bus:GPS-FPL-01-B:state",
  "exists": true,
  "ttl_seconds": 78,
  "raw_fields": {
    "direction": "0",
    "lat": "-20.2443",
    "lon": "57.4828",
    "route_id": "FPL-01",
    "speed_kmh": "40",
    "stop_id": "FPL-S12",
    "stop_index": "11",
    "ts": "1736323855"
  },
  "parsed": {
    "route_id": "FPL-01",
    "direction": 0,
    "stop_index": 11,
    "stop_id": "FPL-S12",
    "lat": -20.2443,
    "lon": 57.4828,
    "speed_kmh": 40,
    "ts": 1736323855,
    "ttl_seconds": 78
  }
}
```

**Use Case:**
- Verify enrichment produced correct route_id, direction, stop_index
- Check Redis TTL (should be 120s by default)
- Compare enriched fields vs raw MQTT to trace transformation

---

### 3. Journey Recommendation Explain
**Purpose:** Shows current recommendation state for a journey with scoring breakdown.

```bash
curl -H "Authorization: Bearer devtoken" \
     -H "X-Dev-Secret: dev" \
     "http://127.0.0.1:9090/api/v1/dev/obs/journey-explain?journeyId=<JID>"
```

**Response:**
```json
{
  "journey_id": "00d614e3-1957-4c6a-8280-27ffcde77a93",
  "user_id": "raj-001",
  "status": 1,
  "status_name": "Tracking",
  "origin_stop_id": "FPL-S02",
  "destination_stop_id": "FPL-S16",
  "required_direction": 0,
  "active_bus_id": "GPS-FPL-01-C",
  "proximity_level": 0,
  "proximity_name": "None",
  "decline_count": 0,
  "recommendations_count": 3,
  "recommendations": [
    {
      "busId": "GPS-FPL-01-C",
      "estimatedArrival": 60000,
      "distanceMeters": 20351.877637520018,
      "direction": 1,
      "isWrongDirection": true,
      "confidenceLevel": 0.6,
      "displayText": "Bus has arrived"
    },
    {
      "busId": "GPS-FPL-01-B",
      "estimatedArrival": 1620000,
      "distanceMeters": 40957.644135683244,
      "direction": 0,
      "isWrongDirection": true,
      "confidenceLevel": 0.40382206439971924,
      "displayText": "Passed stop; loops back (~27 min)"
    },
    {
      "busId": "GPS-FPL-01-A",
      "estimatedArrival": 2250000,
      "distanceMeters": 58902.89173802907,
      "direction": 0,
      "isWrongDirection": true,
      "confidenceLevel": 0.5,
      "displayText": "Passed stop; loops back (~38 min)"
    }
  ],
  "created_at": "2026-02-09T15:28:43.405892Z",
  "updated_at": "2026-02-09T15:28:43.405892Z",
  "message": "Journey tracking state from read model. Shows current top 3 recommendations with scores. For full candidate analysis with PRIMARY/OPPOSITE/PASSED categories, the ranking logic would need to be re-executed."
}
```

**Use Case:**
- See why specific buses rank higher (estimatedArrival, distanceMeters, confidenceLevel)
- Verify active_bus_id matches expected recommendation
- Check isWrongDirection flag for direction validation
- Debug displayText generation logic

---

## Debugging Workflow

### Problem: Journey recommendation mismatch

**Step 1:** Check Redis state for all candidate buses
```bash
for bus in GPS-FPL-01-A GPS-FPL-01-B GPS-FPL-01-C; do
  echo "=== $bus ==="
  curl -s -H "Authorization: Bearer devtoken" -H "X-Dev-Secret: dev" \
    "http://127.0.0.1:9090/api/v1/dev/obs/redis/bus?busId=$bus"
  echo ""
done
```

**Step 2:** Compare with ingestor last event to verify enrichment
```bash
curl -s "http://127.0.0.1:9100/dev/obs/bus?busId=GPS-FPL-01-B"
```

**Questions to answer:**
- Does raw MQTT lat/lon match Redis lat/lon?
- Is route_id correct after enrichment?
- Is stop_index calculated correctly?
- Is direction flag accurate?

**Step 3:** Check journey recommendations
```bash
curl -s -H "Authorization: Bearer devtoken" -H "X-Dev-Secret: dev" \
  "http://127.0.0.1:9090/api/v1/dev/obs/journey-explain?journeyId=<JID>"
```

**Questions to answer:**
- Does ranking order match expected priority (arrival time, distance)?
- Are isWrongDirection flags consistent with required_direction?
- Does displayText reflect correct bus state (arrived, approaching, passed)?
- Is active_bus_id the top recommendation?

**Step 4:** Isolate root cause
- **Enrichment issue:** Ingestor shows correct raw GPS, but Redis has wrong route/stop → check enrichment logic in `cmd/emqx_ingestor`
- **Redis staleness:** Redis TTL expired or not updated → check GPS simulation frequency
- **Ranking issue:** Redis correct, but ranking wrong → check recommendation scoring logic in `application/journey/resolver`

---

## Architecture Diagram

```
┌─────────────┐
│ GPS Device  │
└──────┬──────┘
       │ MQTT: {"lat":-20.245,"lon":57.485,"speed_kmh":40,"timestamp_ms":1770653348135}
       v
┌─────────────────────────┐
│ Ingestor (port 9100)    │ <── Endpoint 1: /dev/obs/bus
│ - Captures raw MQTT     │     Shows: raw_lat, raw_lon, raw_speed, enriched flag
│ - Enriches with route   │
│   stop, direction       │
└──────┬──────────────────┘
       │ Enriched GPS
       v
┌─────────────────────────┐
│ Redis (key-value store) │ <── Endpoint 2: /api/v1/dev/obs/redis/bus
│ - Stores bus state      │     Shows: route_id, stop_index, direction, lat/lon, TTL
│ - TTL: 120s default     │
└──────┬──────────────────┘
       │ Live bus state
       v
┌─────────────────────────┐
│ Ranking/Resolver        │ <── Endpoint 3: /api/v1/dev/obs/journey-explain
│ - Loads buses from Redis│     Shows: recommendations with scores, active_bus_id,
│ - Calculates distances  │           estimatedArrival, confidenceLevel, displayText
│ - Ranks by arrival time │
└─────────────────────────┘
```

---

## Authentication

### Ingestor Endpoint
- **No authentication required** (dev-only port 9100, not exposed in production)

### Simulator Endpoints (Redis + Journey)
- **Bearer token:** `devtoken`
- **Dev secret header:** `X-Dev-Secret: dev`
- These are guarded by `guardDevOnly()` and only available when `IS_DEV=true`

---

## Notes

- **ObsBuffer capacity:** 500 buses (ring buffer, FIFO eviction)
- **Redis TTL:** 120 seconds default (configurable)
- **Journey recommendations:** Shows top 3 from tracking read model, not full candidate list with PRIMARY/OPPOSITE/PASSED categories (would require re-running ranking logic)
- **Enrichment logic:** Located in `cmd/emqx_ingestor/enqueue_handler.go`
- **Ranking logic:** Located in `application/journey/resolver/bus_ranking.go`

---

## Example: Complete Debug Session

```bash
# 1. Start GPS simulation
curl -X POST -H "Authorization: Bearer devtoken" \
  "http://127.0.0.1:9090/api/v1/buses/simulate-all-gps?interpolate=10"

# 2. Check ingestor captured MQTT
curl "http://127.0.0.1:9100/dev/obs/bus?busId=GPS-FPL-01-B"
# Expected: raw_lat, raw_lon, raw_speed from MQTT payload

# 3. Verify Redis enrichment
curl -H "Authorization: Bearer devtoken" -H "X-Dev-Secret: dev" \
  "http://127.0.0.1:9090/api/v1/dev/obs/redis/bus?busId=GPS-FPL-01-B"
# Expected: route_id, stop_index, direction computed correctly

# 4. Get journey ID for user
curl -H "Authorization: Bearer devtoken" \
  "http://127.0.0.1:9090/api/v1/journeys?userId=raj-001"

# 5. Explain journey recommendations
curl -H "Authorization: Bearer devtoken" -H "X-Dev-Secret: dev" \
  "http://127.0.0.1:9090/api/v1/dev/obs/journey-explain?journeyId=<JID>"
# Expected: Top 3 recommendations with scores, active bus selection
```

---

## Related Files

- **ObsBuffer implementation:** `cmd/emqx_ingestor/obs_buffer.go`
- **Ingestor endpoint:** `cmd/emqx_ingestor/obs_handler.go`
- **Redis endpoint:** `internal/presentation/httpapi/handlers_dev_obs_redis.go`
- **Journey endpoint:** `internal/presentation/httpapi/handlers_dev_obs_journey.go`
- **Tracking read model:** `application/journey/trackingreadmodel/contracts.go`
- **Ranking logic:** `application/journey/resolver/bus_ranking.go`
