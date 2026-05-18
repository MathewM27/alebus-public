# Architecture Overview

**Alebus** is a real-time public transit tracking system built with a clean architecture. This document describes the system components, their responsibilities, and how they interact.

---

## System Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        PRESENTATION LAYER                           │
│                                                                     │
│  ┌──────────────────┐           ┌──────────────────┐              │
│  │ simulation_      │  proxies  │   Browser /      │              │
│  │ preview (8081)   │ ────────► │   Mobile Client  │              │
│  └────────┬─────────┘           └──────────────────┘              │
│           │ reverse proxy                                          │
│           │ /api/* → http://127.0.0.1:9090                        │
└───────────┼────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        APPLICATION LAYER                            │
│                                                                     │
│  ┌──────────────────┐           ┌──────────────────┐              │
│  │  simulator       │           │  api             │              │
│  │  (port 9090)     │           │  (port 8080)     │              │
│  │                  │           │                  │              │
│  │ • Dev endpoints  │           │ • Prod-like      │              │
│  │ • MQTT publisher │           │ • No dev         │              │
│  │ • HTTP API       │           │   endpoints      │              │
│  └────────┬─────────┘           └────────┬─────────┘              │
│           │                              │                         │
│           │ publishes GPS                │ reads live state        │
│           │                              │                         │
└───────────┼──────────────────────────────┼─────────────────────────┘
            │                              │
            ▼                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      INFRASTRUCTURE LAYER                           │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                      Message Bus (MQTT)                      │  │
│  │  ┌────────────┐                                              │  │
│  │  │   EMQX     │  Topic: bus/{bus_id}/gps                     │  │
│  │  │  (1883)    │  Payload: {lat, lon, timestamp, ...}         │  │
│  │  └─────┬──────┘                                              │  │
│  │        │ shared subscription: $share/alebus-ingestor/bus/+/gps│  │
│  └────────┼───────────────────────────────────────────────────────┘  │
│           │                                                         │
│           ▼                                                         │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                  Data Enrichment Pipeline                    │  │
│  │  ┌────────────┐                                              │  │
│  │  │  ingestor  │  • Receives raw GPS                          │  │
│  │  │  (9100)    │  • Enriches with route/stop/direction        │  │
│  │  │            │  • Validates freshness (<60s)                │  │
│  │  │            │  • Writes to Redis atomically                │  │
│  │  └─────┬──────┘                                              │  │
│  └────────┼───────────────────────────────────────────────────────┘  │
│           │ HSET bus:{bus_id}:state                                 │
│           ▼                                                         │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                     Live State Store                         │  │
│  │  ┌────────────┐          ┌────────────┐                      │  │
│  │  │   Redis    │ ◄────────┤  journey-  │                      │  │
│  │  │ (6382→6379)│          │  worker    │                      │  │
│  │  │            │          │            │                      │  │
│  │  │ • bus:*    │          │ • Reads    │                      │  │
│  │  │ • journey:*│          │   pubsub   │                      │  │
│  │  │ • pubsub   │          │ • Updates  │                      │  │
│  │  │            │          │   journeys │                      │  │
│  │  └─────▲──────┘          └────────────┘                      │  │
│  │        │ reads (API, simulator, journey-worker)              │  │
│  └────────┼───────────────────────────────────────────────────────┘  │
│           │                                                         │
│  ┌────────┼──────────────────────────────────────────────────────┐  │
│  │        │              Persistent Storage                      │  │
│  │  ┌─────▼──────┐                                               │  │
│  │  │ PostgreSQL │  Tables: routes, stops, buses, users,        │  │
│  │  │  (5432)    │          journeys, bus_devices, etc.         │  │
│  │  │ PostGIS    │  Extensions: postgis, uuid-ossp              │  │
│  │  └────────────┘                                               │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities

### Presentation Layer

#### simulation_preview (port 8081)
**Role:** Local development UI and reverse proxy.

**Responsibilities:**
- Serves static HTML/JS/CSS for testing UI
- Reverse-proxies `/api/*` traffic to upstream API (default: `http://127.0.0.1:9090`)
- Enables SSE streams for real-time bus updates
- Does NOT run application logic itself

**Configuration:**
- `SIM_PREVIEW_API_BASE_URL`: Upstream API target (default: `http://127.0.0.1:9090`)
- `SIM_PREVIEW_MODE`: `proxy` (default) or `embedded` (legacy)

**Why it exists:** Allows UI to be served on same origin as API (avoids CORS issues), enables SSE to work cleanly in browser.

---

### Application Layer

#### simulator (port 9090)
**Role:** Development API server with MQTT publishing capabilities.

**Responsibilities:**
- Serves all standard HTTP API routes (`/api/v1/*`)
- Exposes dev-only endpoints (`/api/v1/dev/*`) for testing
- Acts as MQTT client to publish GPS data to EMQX
- Reads from PostgreSQL for route/bus/user data
- Reads from Redis for live bus state

**Configuration:**
- `SIMULATOR_PORT=9090`
- `DEV_ENDPOINT_SECRET=dev`
- `DEV_ENDPOINT_ALLOWED_IPS=127.0.0.1/32,...`
- `MQTT_BROKER_URL=mqtt://emqx:1883`
- `DATABASE_URL=postgres://alebus:alebus@postgres:5432/alebus`
- `REDIS_ADDR=redis:6379`

**Why it exists:** Combines API serving with GPS simulation in one binary for fast local iteration. Dev endpoints allow controlled testing of edge cases.

---

#### api (port 8080)
**Role:** Production-like API server.

**Responsibilities:**
- Serves all standard HTTP API routes (`/api/v1/*`)
- Reads from PostgreSQL for route/bus/user data
- Reads from Redis for live bus state
- Does NOT have dev endpoints
- Does NOT publish to MQTT

**Configuration:**
- `API_HTTP_PORT=8080`
- `API_AUTH_MODE=off` (dev stack), `jwt` (production)
- `DATABASE_URL=postgres://alebus:alebus@postgres:5432/alebus`
- `REDIS_ADDR=redis:6379`

**Why it exists:** Validates that the API works without dev-specific features. Used for integration testing before production deployment.

**Key difference from simulator:** No MQTT publishing, no dev endpoints. Otherwise serves identical routes.

---

### Infrastructure Layer

#### postgres (port 5432)
**Role:** Persistent data store.

**Responsibilities:**
- Stores routes, stops, buses, users, journeys
- Provides PostGIS extensions for geospatial queries
- Serves as source of truth for static/configuration data

**Schema:** See `infrastructure/migrations/*.up.sql`

**Why PostGIS:** Required for `ST_Distance`, `ST_DWithin`, and other geospatial operations on stop locations.

---

#### redis (port 6382 → 6379)
**Role:** Live state authoritative store.

**Responsibilities:**
- Stores real-time bus state (`bus:{bus_id}:state`)
- Stores real-time journey state (`journey:{journey_id}`)
- Publishes to pubsub channels for journey updates
- Provides heartbeat mechanism (TTL on bus keys)

**Topology:** Standalone (single instance, no Sentinel). Suitable for dev/staging.

**Why Redis:** Sub-millisecond read latency required for journey recommendations. PostgreSQL too slow for real-time reads at scale.

**Port mapping:** Host port 6382 prevents conflicts with legacy Sentinel stack (which would use 6379). Internal services use `redis:6379` (container network).

---

## Realtime Transport

**Alebus uses WebSockets for all user-facing realtime updates.**

**Active WebSocket endpoints:**
- `/api/v1/ws/journeys` — Journey-specific bus + journey updates
- `/api/v1/ws/live-buses` — Live bus position stream
- `/api/v1/ws/mux` — Multiplexed WebSocket (recommended for production)

**Why WebSockets:**
- Mobile compatible (React Native Expo)
- Bidirectional communication
- Connection multiplexing (one TCP connection, multiple subscriptions)
- Lower latency than HTTP polling

**SSE (Server-Sent Events) status:** Deprecated for user-facing features. Legacy SSE endpoint (`/api/v1/stream/live-buses`) exists but is being phased out.

**See:** [REALTIME_TRANSPORT.md](./REALTIME_TRANSPORT.md) for complete WebSocket documentation.

---

#### emqx (ports 1883, 18083)
**Role:** MQTT message broker.

**Responsibilities:**
- Receives GPS messages on topic `bus/{bus_id}/gps`
- Distributes messages to subscribers (ingestor)
- Provides dashboard for monitoring (port 18083)
- Supports shared subscriptions for horizontal scaling

**Configuration:**
- `EMQX_ALLOW_ANONYMOUS=true` (dev stack)
- `EMQX_AUTHENTICATION__1__ENABLE=false` (dev stack)
- Shared subscription strategy: `round_robin`

**Why EMQX:** MQTT is standard protocol for IoT/GPS devices. EMQX supports clustering and shared subscriptions.

---

#### ingestor (port 9100)
**Role:** MQTT → Redis data enrichment bridge.

**Responsibilities:**
- Subscribes to `$share/alebus-ingestor/bus/+/gps` (shared subscription)
- Validates GPS freshness (<60 seconds)
- Enriches GPS with route/stop/direction context (Phase 5 logic)
- Writes enriched state to Redis atomically
- Exposes health endpoint (`/healthz`)

**Configuration:**
- `EMQX_TOPIC_FILTER=bus/+/gps`
- `EMQX_SHARED_GROUP=alebus-ingestor`
- `ENABLE_GPS_ENRICHMENT=true`
- `GPS_ROLLOUT_PERCENTAGE=100`
- `INGESTOR_DROP_POLICY=coalesce_latest`

**Why shared subscription:** Allows multiple ingestor instances to share load (horizontal scaling).

**Why enrichment here:** Decouples GPS ingestion from API serving. API reads pre-enriched state from Redis.

---

#### journey-worker (no exposed port)
**Role:** Journey state machine background worker.

**Responsibilities:**
- Polls active journeys from PostgreSQL (every 2 seconds)
- Reads live bus state from Redis
- Computes journey updates (arrival estimates, bus recommendations)
- Writes updated journey state to Redis
- Publishes journey update events to Redis pubsub

**Configuration:**
- `JOURNEY_WORKER_INTERVAL=2s`
- `JOURNEY_WORKER_BATCH_SIZE=500`
- `DATABASE_URL=postgres://alebus:alebus@postgres:5432/alebus`
- `REDIS_ADDR=redis:6379`

**Why a separate worker:** Journey recommendations require iterative scoring across multiple buses. Running this in the API request path would cause unacceptable latency.

**Scaling:** Compose profile `journey-worker-sharded` allows running 4 workers with shard-based partitioning (currently disabled in dev stack).

---

## Layer Separation

### Presentation Layer
- **What:** UI, proxies, clients
- **Responsibilities:** Rendering, user interaction, API consumption
- **Examples:** simulation_preview, mobile apps, web dashboards

### Application Layer
- **What:** Business logic, HTTP API serving
- **Responsibilities:** Request validation, authorization, orchestration, response formatting
- **Examples:** simulator, api
- **Key rule:** Application layer does NOT directly write to Redis. Only reads for serving responses.

### Infrastructure Layer
- **What:** Data storage, message brokering, background workers
- **Responsibilities:** Persistence, pub/sub, data enrichment, state updates
- **Examples:** postgres, redis, emqx, ingestor, journey-worker
- **Key rule:** Infrastructure layer owns writes to Redis. Application layer only reads.

---

## Why Two APIs (simulator + api)?

**simulator (9090):**
- **Primary use:** Local development
- **Features:** Dev endpoints enabled, MQTT publishing, full API
- **Target:** `simulation_preview` default proxy target

**api (8080):**
- **Primary use:** Production-like integration testing
- **Features:** No dev endpoints, no MQTT, full API
- **Target:** CI/CD pipelines, staging environments

**Why maintain both:**
1. **Separation of concerns:** Dev endpoints should not leak into production
2. **Binary validation:** Ensures prod binary works without dev features
3. **MQTT isolation:** Simulator can publish GPS without affecting prod API

**Which to use:**
- **Local dev:** Always use simulator (9090)
- **Integration tests:** Use api (8080) to validate prod-like behavior
- **Production:** Deploy api binary only

---

## Why Redis is Authoritative for Live State

**Problem:** PostgreSQL is too slow for real-time bus queries at scale.

**Solution:** Redis stores live state with sub-millisecond read latency.

**Trade-offs:**
- **Pro:** Fast reads (~0.1ms vs ~10ms for PostgreSQL)
- **Pro:** Automatic expiration (TTL) for stale bus state
- **Pro:** Pubsub for real-time updates (SSE streams)
- **Con:** Not persistent (bus state lost on restart)
- **Con:** No transactions (requires careful key design)

**Mitigation:** PostgreSQL stores journey history and user data (persistent). Redis only stores ephemeral live state.

**Key design rule:** Redis is a cache with a TTL. If data must survive restarts, it goes in PostgreSQL.

---

## Data Flow Summary

1. **GPS source** (simulator or device) → publishes to `bus/{bus_id}/gps` on EMQX
2. **EMQX** → distributes to ingestor via shared subscription
3. **Ingestor** → enriches GPS, writes to Redis `bus:{bus_id}:state`
4. **Journey-worker** → reads Redis, computes journey updates, writes to Redis `journey:{journey_id}`
5. **API/Simulator** → reads Redis for live state, serves HTTP responses
6. **simulation_preview** → proxies to simulator (9090), renders UI

**Critical path latency:** GPS → EMQX → ingestor → Redis: **<50ms**  
**Journey update latency:** Redis → journey-worker → Redis: **2-4s** (batch interval)

---

## Next Steps

- **Understand data flow:** [RUNTIME_DATA_FLOW.md](./RUNTIME_DATA_FLOW.md)
- **Understand Phase 2 scope:** [PHASE_2_SCOPE.md](./PHASE_2_SCOPE.md)
- **Quick start:** [DEV_QUICKSTART.md](./DEV_QUICKSTART.md)
