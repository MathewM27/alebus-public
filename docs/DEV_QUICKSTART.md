# Developer Quick Start

**Alebus** is a real-time public transit tracking platform. It ingests GPS data via MQTT, enriches it with route/stop context, stores live state in Redis, and serves journey recommendations via HTTP API. The system uses PostgreSQL for persistent data, EMQX for message brokering, and a journey-worker for state machine updates.

---

## Prerequisites

Install these tools before starting:

1. **Docker** (with Docker Compose v2+)
2. **Make** (GNU Make or compatible)
3. **Go 1.21+** (for running simulation_preview UI)
4. **golang-migrate CLI** (for database migrations)

**Install golang-migrate** (WSL/Linux):
```bash
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/migrate
chmod +x /usr/local/bin/migrate
```

---

## Start the Dev Stack

```bash
# 1. Start all services
make dev-up

# 2. Run database migrations (first time only)
export DATABASE_URL="postgres://alebus:alebus@localhost:5432/alebus?sslmode=disable"
make migrate-up

# 3. Verify all services are healthy
make dev-verify
```

**What just started:**
- postgres (database)
- redis (live bus state)
- emqx (MQTT broker)
- ingestor (MQTT → Redis bridge)
- journey-worker (journey state machine)
- simulator (dev API on port 9090)
- api (prod-like API on port 8080)

---

## Port Reference

| Port | Service | Purpose | Primary Use |
|------|---------|---------|-------------|
| **9090** | **simulator** | **Dev API + MQTT publisher + WebSocket** | **Default target for simulation_preview** |
| 8080 | api | Prod-like API (no dev endpoints) + WebSocket | Integration testing |
| 8081 | simulation_preview | UI reverse proxy | Local testing UI |
| 5432 | postgres | Database (PostGIS-enabled) | Persistent storage |
| 6382 | redis | Live bus state (host port) | Real-time reads/writes |
| 1883 | emqx | MQTT broker (TCP) | GPS ingestion |
| 18083 | emqx | EMQX dashboard | MQTT monitoring |
| 9100 | ingestor | Health/metrics endpoint | Ingestor health checks |

**Key Rule:** `simulation_preview` proxies to **port 9090** (simulator) by default. Use simulator for all local development.

**WebSocket Endpoints:**
- `ws://127.0.0.1:9090/api/v1/ws/mux` — Multiplexed WebSocket (recommended)
- `ws://127.0.0.1:9090/api/v1/ws/journeys` — Journey-specific WebSocket
- `ws://127.0.0.1:9090/api/v1/ws/live-buses` — Live bus positions

**See:** [REALTIME_TRANSPORT.md](./REALTIME_TRANSPORT.md) for WebSocket documentation.

---

## Run the UI

```bash
cd simulation_preview
go run .
```

**Opens at:** http://127.0.0.1:8081

**What it does:**
- Serves static UI files (HTML/JS)
- Reverse-proxies `/api/*` requests to `http://127.0.0.1:9090` (simulator)
- Enables SSE streams for real-time bus updates

**To target the prod-like API instead (port 8080):**
```bash
export SIM_PREVIEW_API_BASE_URL=http://127.0.0.1:8080
cd simulation_preview
go run .
```

---

## Verify Health

```bash
# Show running containers and ports
make dev-status

# Test all health endpoints
make dev-verify

# Tail logs from all services
make dev-logs
```

**Expected output from `make dev-verify`:**
```
1. Postgres: alebus UP
2. Redis: PONG
3. EMQX: EMQX dashboard accessible
4. Simulator API (9090): Simulator health OK
5. API (8080): API health OK
```

---

## Stop the Stack

```bash
make dev-down
```

**To remove all data volumes (clean slate):**
```bash
make clean
```

---

## Common Mistakes

### ❌ Running legacy compose commands

**Wrong:**
```bash
docker compose -f docker-compose.yml -f docker-compose.emqx.yml up
```

**Right:**
```bash
make dev-up
```

**Why:** Old compose files are deleted. The canonical stack is in `compose/dev.yml`.

---

### ❌ Targeting the wrong port

**Wrong:**
```bash
export SIM_PREVIEW_API_BASE_URL=http://127.0.0.1:8080
# This points to the prod-like API (no dev endpoints)
```

**Right:**
```bash
# Use default (no export needed)
# simulation_preview targets http://127.0.0.1:9090 by default
```

**Why:** Port 9090 (simulator) has dev endpoints enabled. Port 8080 (api) does not.

---

### ❌ Port collision errors

**Symptom:**
```
Error: Bind for 0.0.0.0:9090 failed: port is already allocated
```

**Cause:** Old containers still running from previous compose setup.

**Fix:**
```bash
# Remove old containers
docker rm -f $(docker ps -aq --filter "name=alebus-")

# Restart dev stack
make dev-up
```

---

### ❌ Database table errors

**Symptom:**
```
ERROR: relation "routes" does not exist (SQLSTATE 42P01)
```

**Cause:** Migrations haven't been run.

**Fix:**
```bash
export DATABASE_URL="postgres://alebus:alebus@localhost:5432/alebus?sslmode=disable"
make migrate-up
```

---

## Next Steps

1. **Load sample routes**: Use simulation_preview UI → "Load Sample Routes"
2. **Create buses**: Use simulation_preview UI → "Create Bus"
3. **Publish GPS data**: Use simulation_preview UI → "Start GPS Simulation"
4. **Watch SSE streams**: Open http://127.0.0.1:8081/sse_debug.html
5. **Read architecture docs**: See [ARCHITECTURE_OVERVIEW.md](./ARCHITECTURE_OVERVIEW.md)

---

## Development Workflow

```bash
# Morning startup
make dev-up
make dev-verify

# During development
make dev-logs              # Monitor all services
make db-shell              # Inspect database
docker exec alebus-redis-emqx redis-cli -p 6379 KEYS 'bus:*'  # Inspect Redis

# Evening shutdown
make dev-down
```

---

## Troubleshooting

**Check container status:**
```bash
docker ps --filter "name=alebus-"
```

**Inspect specific service logs:**
```bash
docker logs -f alebus-simulator
docker logs -f alebus-ingestor
docker logs -f alebus-journey-worker
```

**Verify Redis connectivity:**
```bash
docker exec alebus-redis-emqx redis-cli -p 6379 ping
# Expected: PONG
```

**Verify EMQX connectivity:**
```bash
curl -s http://127.0.0.1:18083
# Expected: HTTP 200 + EMQX dashboard HTML
```

**Verify database connectivity:**
```bash
docker exec alebus-postgres pg_isready -U alebus -d alebus
# Expected: alebus:5432 - accepting connections
```

---

## Reference

- **Canonical compose file**: `compose/dev.yml`
- **Legacy files (deprecated)**: `compose/legacy/`
- **Makefile targets**: `make help`
- **Architecture**: [ARCHITECTURE_OVERVIEW.md](./ARCHITECTURE_OVERVIEW.md)
- **Data flow**: [RUNTIME_DATA_FLOW.md](./RUNTIME_DATA_FLOW.md)
