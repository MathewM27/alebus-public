# Runtime Data Flow

This document traces GPS data from source to browser, explaining transformations, filters, and business logic at each step.

---

## Overview

```
GPS Device/Simulator
    ↓ MQTT publish
EMQX Broker
    ↓ shared subscription
Ingestor (enrichment + validation)
    ↓ atomic write
Redis (authoritative live state)
    ↓ polling (journey-worker)
    ↓ read (API/simulator)
HTTP Response
    ↓ reverse proxy
simulation_preview
    ↓ SSE stream
Browser
```

---

## Step 1: GPS Source → EMQX

### Source

**Options:**
1. **simulator (dev):** Publishes synthetic GPS via MQTT client (`API_MQTT_CLIENT_ID=alebus-simulator`)
2. **Physical GPS device (prod):** Publishes real GPS data via MQTT

**Message format:**
```json
{
  "bus_id": "BUS-001",
  "lat": 40.7128,
  "lon": -74.0060,
  "timestamp": "2026-02-04T16:00:00Z",
  "speed": 45.5,
  "heading": 180.0
}
```

**MQTT topic:** `bus/{bus_id}/gps`

**Example:**
```
bus/BUS-001/gps → {lat: 40.7128, lon: -74.0060, ...}
```

---

### EMQX Broker

**Role:** Message distribution hub.

**What happens:**
1. EMQX receives message on `bus/BUS-001/gps`
2. Checks subscriber list for topic pattern `bus/+/gps`
3. Finds ingestor subscribed to `$share/alebus-ingestor/bus/+/gps`
4. Routes message to one ingestor instance (round-robin if multiple)

**Shared subscription benefits:**
- Multiple ingestors can share load
- EMQX handles distribution automatically
- No need for external load balancer

**QoS level:** 1 (at least once delivery)

---

## Step 2: EMQX → Ingestor

### Ingestor Responsibilities

**Phase 1: Receive**
```go
// Subscribe to shared topic
client.Subscribe("$share/alebus-ingestor/bus/+/gps", 1, handler)
```

**Phase 2: Validate Freshness**
```go
now := time.Now()
msgAge := now.Sub(gps.Timestamp)

if msgAge > 60*time.Second {
    log.Printf("[GPS] REJECTED bus_id=%s reason=stale age=%v", busID, msgAge)
    return // Drop stale messages
}
```

**Critical rule:** GPS older than 60 seconds is discarded. Prevents outdated data from polluting live state.

---

### Phase 3: GPS Enrichment (Phase 5 Logic)

**Inputs:**
- Raw GPS: `{lat, lon, timestamp, speed, heading}`
- Route data: From PostgreSQL (`routes`, `stops` tables)

**Enrichment process:**
1. **Route matching:** Find which route this bus is assigned to (lookup in `buses` table)
2. **Stop proximity:** Calculate distance to all stops on route using PostGIS `ST_Distance`
3. **Direction inference:** Determine route direction (A→B or B→A) based on heading and stop sequence
4. **Stop index:** Find closest stop index in route sequence
5. **Speed filtering:** If speed < 5 km/h and near terminal → mark as `dwelling`

**Output (enriched GPS):**
```json
{
  "bus_id": "BUS-001",
  "route_id": "ROUTE-A",
  "direction": "outbound",
  "stop_index": 12,
  "is_terminal": false,
  "lat": 40.7128,
  "lon": -74.0060,
  "speed": 45.5,
  "heading": 180.0,
  "timestamp": "2026-02-04T16:00:00Z",
  "enriched_at": "2026-02-04T16:00:00.123Z"
}
```

**Why enrich here (not in API):**
- Decouples ingestion from serving
- API reads pre-computed values (fast reads)
- Enrichment logic runs once per GPS update (not per API request)

---

## Step 3: Ingestor → Redis

### Redis Write

**Key structure:**
```
bus:{bus_id}:state
```

**Command:**
```redis
HSET bus:BUS-001:state
  route_id "ROUTE-A"
  direction "outbound"
  stop_index "12"
  is_terminal "false"
  lat "40.7128"
  lon "-74.0060"
  speed "45.5"
  heading "180.0"
  timestamp "2026-02-04T16:00:00Z"
  enriched_at "2026-02-04T16:00:00.123Z"
```

**TTL (expiration):**
```redis
EXPIRE bus:BUS-001:state 300
```

**Why TTL:** If bus stops sending GPS for 5 minutes, its state automatically expires. Prevents stale buses from appearing in recommendations.

---

### Atomicity

**Problem:** Multiple GPS updates for same bus arrive concurrently.

**Solution:** Redis is single-threaded. `HSET` commands are atomic.

**Result:** Last-write-wins semantics. Most recent GPS update overwrites previous state.

---

## Step 4: Redis → Journey Worker

### Journey Worker Responsibilities

**Role:** Background worker that computes journey recommendations.

**Polling loop:**
```go
for {
    time.Sleep(2 * time.Second)

    // 1. Fetch active journeys from PostgreSQL
    journeys := db.Query("SELECT * FROM journeys WHERE status = 'active'")

    // 2. For each journey, read live bus state from Redis
    for _, journey := range journeys {
        buses := redis.MGet(fmt.Sprintf("bus:*:state"))

        // 3. Filter buses for this journey's route
        routeBuses := filterByRoute(buses, journey.RouteID)

        // 4. Score buses (distance, ETA, freshness)
        recommendations := scoreBuses(routeBuses, journey)

        // 5. Write recommendations to Redis
        redis.HSet(fmt.Sprintf("journey:%s", journey.ID), "recommendations", recommendations)

        // 6. Publish update event to pubsub
        redis.Publish(fmt.Sprintf("journey:%s:updates", journey.ID), "bus.update")
    }
}
```

**Batch size:** Processes up to 500 journeys per iteration.

**Why polling:** Simpler than event-driven. 2-second latency is acceptable for journey updates.

---

### Recommendations Scoring

**Inputs:**
- Journey start location (`journey.start_lat`, `journey.start_lon`)
- Journey route ID (`journey.route_id`)
- Live bus states from Redis

**Filters:**
1. **Route match:** Only buses on `journey.route_id`
2. **Direction match:** Only buses traveling toward destination
3. **Freshness:** Only buses with GPS updates in last 60 seconds

**Critical: No status filter**

**Previous bug (Phase 5):** System filtered out buses with `status != 'Active'`.

**Problem:** Buses dwelling at terminals have `status = 'Idle'` but are still valid candidates.

**Fix:** Removed status filter. Now scoring is based on:
- **Distance:** How close is the bus to the user?
- **ETA:** When will it arrive at nearest stop?
- **Freshness:** Is GPS data recent?

**Why freshness matters:**
- Bus with GPS from 5 seconds ago: High confidence
- Bus with GPS from 55 seconds ago: Lower confidence (might have moved)
- Bus with GPS from 61+ seconds ago: Excluded (expired TTL)

---

### Speed vs Status Semantics

**Status field (in PostgreSQL `buses` table):**
- `Active`: Bus is in service and moving
- `Idle`: Bus is stationary (dwelling at terminal, waiting at stop)
- `Maintenance`: Bus is out of service
- `Inactive`: Bus is not scheduled

**Speed field (in Redis `bus:{id}:state`):**
- `> 5 km/h`: Bus is moving
- `< 5 km/h`: Bus is stationary (might be dwelling, might be in traffic)

**Key insight:** A bus can be `Idle` (status) but still be a valid recommendation if it's dwelling at a terminal and will resume service soon.

**Design decision:** Journey recommendations use **speed** and **position**, not **status**.

---

## Step 5: Redis → API/Simulator

### API Read Path

**Endpoint:** `GET /api/v1/journeys/{journey_id}`

**Logic:**
```go
// 1. Fetch journey metadata from PostgreSQL
journey := db.QueryRow("SELECT * FROM journeys WHERE id = ?", journeyID)

// 2. Fetch live recommendations from Redis
redisKey := fmt.Sprintf("journey:%s", journeyID)
recommendations := redis.HGet(redisKey, "recommendations")

// 3. Merge and return
response := JourneyResponse{
    ID: journey.ID,
    UserID: journey.UserID,
    RouteID: journey.RouteID,
    Status: journey.Status,
    Recommendations: recommendations,  // From Redis
    CreatedAt: journey.CreatedAt,      // From PostgreSQL
}

return response
```

**Why two sources:**
- **PostgreSQL:** Journey configuration (static, persistent)
- **Redis:** Live recommendations (dynamic, ephemeral)

**Read latency:**
- PostgreSQL: ~5-10ms
- Redis: ~0.1-1ms
- Total: ~10ms

---

### Realtime Update Delivery

**Journey-worker publishes to Redis pubsub:**
```go
redis.Publish(fmt.Sprintf("journey:%s:updates", journeyID), "bus.update")
```

**WebSocket handler subscribes and forwards:**
```go
updates := journeyUpdatesSubscriber.Subscribe(ctx, journeyID)

for upd := range updates {
    msg := WSMessage{
        Type: "journey.update",
        Data: upd.Journey,
    }
    conn.WriteJSON(msg) // Push to WebSocket
}
```

**Client receives:**
```json
{
  "type": "journey.update",
  "data": {
    "journeyId": "123",
    "activeBusId": "BUS-001",
    "estimatedArrival": "2026-02-04T12:15:00Z"
  }
}
```

**Why WebSocket:** Mobile-compatible, lower latency, bidirectional communication support.

---

## Step 6: API → WebSocket → Browser

### WebSocket Connection (Realtime Updates)

**Modern approach (WebSocket):**

**Endpoint:** `ws://127.0.0.1:9090/api/v1/ws/mux`

**Logic:**
```javascript
const ws = new WebSocket('ws://127.0.0.1:9090/api/v1/ws/mux');

ws.onopen = () => {
  // Subscribe to journey updates
  ws.send(JSON.stringify({
    type: 'subscribe',
    data: {
      stream: 'journeys',
      journeyId: '123',
    },
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  
  if (msg.type === 'bus.update') {
    // Update bus position on map
    updateBusMarker(msg.data.bus);
  }
  
  if (msg.type === 'journey.update') {
    // Update journey state
    updateJourneyStatus(msg.data.journey);
  }
};
```

**Why WebSocket:** Mobile-compatible (React Native Expo), bidirectional, multiplexing support.

**SSE (deprecated):** Legacy endpoint `/api/v1/stream/journeys` still exists but should not be used for new implementations.

---

### Reverse Proxy Path (HTTP API Calls)

**simulation_preview configuration:**
```go
cfg.APIBaseURL = "http://127.0.0.1:9090"  // Default
proxy := httputil.NewSingleHostReverseProxy(targetURL)
mux.Handle("/api/", proxy)
```

**Request flow:**
1. Browser → `http://127.0.0.1:8081/api/v1/journeys/123`
2. simulation_preview → `http://127.0.0.1:9090/api/v1/journeys/123` (proxy)
3. simulator (9090) → processes request, returns response
4. simulation_preview → forwards response to browser

**Why proxy:** Avoids CORS issues. Browser sees same origin (8081).

---

### WebSocket Message in Browser

**JavaScript:**
```javascript
const ws = new WebSocket('ws://127.0.0.1:9090/api/v1/ws/mux');

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'subscribe',
    data: { stream: 'journeys', journeyId: '123' },
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  
  if (msg.type === 'bus.update') {
    updateBusMarker(msg.data.bus);
  }
  
  if (msg.type === 'journey.update') {
    updateJourneyDisplay(msg.data.journey);
  }
};
```

**Real-time flow:**
1. Journey-worker updates Redis every 2 seconds
2. Journey-worker publishes to `journey:{id}:updates` pubsub
3. WebSocket handler receives pubsub message
4. WebSocket handler fetches latest state from Redis
5. WebSocket handler pushes JSON message to client
6. Browser JavaScript updates UI

**End-to-end latency:** GPS update → browser UI: **2-4 seconds**

---

## Data Consistency Model

### Eventual Consistency

**Problem:** Redis and PostgreSQL are separate stores.

**Example:**
1. Journey created in PostgreSQL at `T=0`
2. Journey-worker polls PostgreSQL at `T=2s`
3. Recommendations appear in Redis at `T=2s`

**Result:** 2-second delay before recommendations are available.

**Acceptable:** Journey creation is not real-time critical. User expects a few seconds of setup time.

---

### Race Conditions

**Scenario:** Two GPS updates arrive for same bus within 1ms.

**Mitigation:** Redis is single-threaded. Last write wins.

**Result:** Most recent GPS update is authoritative. Older update is discarded.

**Impact:** None. GPS updates are idempotent.

---

## Failure Modes

### EMQX Down

**Symptom:** No GPS updates reach ingestor.

**Impact:**
- Existing Redis state remains (TTL not expired)
- After 5 minutes, all bus states expire (TTL=300s)
- Recommendations stop updating

**Recovery:** Restart EMQX. GPS devices reconnect automatically.

---

### Ingestor Down

**Symptom:** GPS messages queue in EMQX.

**Impact:**
- Messages delivered when ingestor restarts (QoS=1)
- Redis state goes stale (TTL expires)
- Recommendations disappear

**Recovery:** Restart ingestor. Processes queued messages.

---

### Redis Down

**Symptom:** Ingestor cannot write. API cannot read.

**Impact:**
- All live state lost
- Recommendations unavailable
- SSE streams break

**Recovery:** Restart Redis. Live state rebuilds as GPS updates arrive.

**Data loss:** All ephemeral state lost. PostgreSQL data intact.

---

### Journey-Worker Down

**Symptom:** Recommendations stop updating.

**Impact:**
- Existing recommendations remain in Redis (stale)
- No new recommendations computed
- SSE streams receive no new events

**Recovery:** Restart journey-worker. Resumes polling immediately.

---

## Performance Characteristics

### Throughput

**GPS ingestion:**
- Ingestor: 1000 messages/second per instance
- EMQX: 100,000 messages/second (clustering enabled)

**API reads:**
- Redis reads: 10,000 req/s per instance
- PostgreSQL reads: 1,000 req/s per instance

**Journey updates:**
- Journey-worker: 500 journeys per 2-second batch
- Effective rate: 250 journey updates/second

---

### Latency

**Critical path (GPS → Redis):**
- EMQX → ingestor: <10ms
- Ingestor enrichment: <30ms
- Redis write: <5ms
- **Total: <50ms**

**API read path:**
- Redis read: <1ms
- PostgreSQL read: <10ms
- HTTP serialization: <5ms
- **Total: <20ms**

**Journey update path:**
- Journey-worker polling: 2s interval
- Redis read/write: <10ms
- Pubsub publish: <5ms
- **Total: 2-4s**

---

## Next Steps

- **Understand architecture:** [ARCHITECTURE_OVERVIEW.md](./ARCHITECTURE_OVERVIEW.md)
- **Understand Phase 2 scope:** [PHASE_2_SCOPE.md](./PHASE_2_SCOPE.md)
- **Quick start:** [DEV_QUICKSTART.md](./DEV_QUICKSTART.md)
