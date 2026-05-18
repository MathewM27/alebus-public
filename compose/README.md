# Alebus Docker Compose Configuration

This directory contains Docker Compose configurations for the Alebus platform.

## Canonical Dev Stack

**File**: [`dev.yml`](./dev.yml)

The **canonical development stack** for local development. This is the recommended way to run Alebus locally.

### Quick Start

```bash
# From repo root
make dev-up      # Start dev stack
make dev-status  # Check status
make dev-verify  # Verify all services healthy
make dev-logs    # Tail logs
make dev-down    # Stop stack
```

### Services Included

| Service | Container Name | Host Port | Purpose |
|---------|----------------|-----------|---------|
| **simulator** | `alebus-simulator` | **9090** | **Dev API with dev endpoints** (default for simulation_preview) |
| api | `alebus-api` | 8080 | Prod-like API (optional, for testing auth/prod behavior) |
| postgres | `alebus-postgres` | 5432 | Database (PostGIS) |
| redis | `alebus-redis-emqx` | 6382 → 6379 | Live bus state (standalone, dev-optimized) |
| emqx | `alebus-emqx` | 1883, 18083 | MQTT broker + dashboard |
| ingestor | `alebus-ingestor` | 9100 | MQTT → Redis bridge (GPS enrichment) |
| journey-worker | `alebus-journey-worker` | (internal) | Journey tracking + PubSub publisher |

### Profiles

- **`journey-worker`** (default): Enables single journey worker (recommended for dev)
- **`journey-worker-sharded`**: Enables 4 sharded journey workers (production-like, advanced testing)

### Manual Usage

```bash
# Start with default journey-worker profile
docker compose -f compose/dev.yml --profile journey-worker up -d --build

# Start with sharded workers (advanced)
docker compose -f compose/dev.yml --profile journey-worker-sharded up -d --build

# Stop
docker compose -f compose/dev.yml --profile journey-worker down
```

### simulation_preview Setup

The `simulation_preview` UI runs locally (not in Docker) and proxies API calls to the dev stack.

**Default configuration**:
- Proxies to `http://127.0.0.1:9090` (simulator)
- UI available at `http://127.0.0.1:8081`

**To run**:
```bash
cd simulation_preview
go run .
```

**To test prod-like API** (port 8080):
```bash
export SIM_PREVIEW_API_BASE_URL=http://127.0.0.1:8080
cd simulation_preview
go run .
```

### Port Reference

| Port | Service | Notes |
|------|---------|-------|
| **9090** | **simulator** | **Primary dev API** |
| 8080 | api | Prod-like testing |
| 5432 | postgres | Database |
| 6382 | redis | Host port (maps to 6379 inside container) |
| 1883 | emqx | MQTT TCP |
| 18083 | emqx | Dashboard (http://127.0.0.1:18083) |
| 9100 | ingestor | Health/metrics |

### Environment Variables

All services use sensible defaults. Optional overrides:

```bash
# Database credentials (defaults: alebus/alebus/alebus)
export POSTGRES_DB=alebus
export POSTGRES_USER=alebus
export POSTGRES_PASSWORD=alebus

# EMQX security (defaults: anonymous allowed)
export EMQX_ALLOW_ANONYMOUS=true
```

---

## Legacy Configurations

**Directory**: [`legacy/`](./legacy/)

Legacy compose files for reference only. **Not recommended for local dev**.

### docker-compose.sentinel.yml

Redis Sentinel HA setup (1 master + 2 replicas + 3 sentinels). For production-like testing only.

**Deprecated**: Use `dev.yml` for local development.

**Warning**: Do NOT run sentinel stack simultaneously with dev stack — causes port collisions and Redis topology conflicts.

### docker-compose.observability.yml

Observability stack (Prometheus + Grafana + custom metrics exporter).

**Deprecated**: Use `dev.yml` for local development.

**Critical Warning**: Exposes Prometheus on port **9090**, which collides with `alebus-simulator`. Do NOT run this stack with dev stack.

**TODO**: Update observability to use a different Prometheus port (e.g., 9091) when re-enabled.

---

## Migration from Legacy Setup

**Old way** (scattered compose files in root):
```bash
docker compose -f docker-compose.yml \
  -f docker-compose.emqx.yml \
  -f docker-compose.api.yml \
  --profile journey-worker up -d --build
```

**New way** (canonical dev stack):
```bash
make dev-up
# OR
docker compose -f compose/dev.yml --profile journey-worker up -d --build
```

**Benefits**:
- Single command
- Clear service ownership
- Port collision detection (via `make dev-preflight`)
- No confusion about which redis/API/stack is running

---

## Troubleshooting

### Port Collision

**Error**: `Error response from daemon: driver failed programming external connectivity on endpoint ... Bind for 0.0.0.0:9090 failed: port is already allocated`

**Cause**: Another container or process is using port 9090 (likely Prometheus from observability stack).

**Solution**:
```bash
# Check what's using the port
docker ps --filter "publish=9090"

# If observability stack is running, stop it
docker compose -f compose/legacy/docker-compose.observability.yml down

# Then retry dev stack
make dev-up
```

### Redis Topology Conflict

**Error**: Safety check fails with "Both standalone and sentinel Redis stacks are running!"

**Cause**: Both `alebus-redis-emqx` (standalone) and `alebus-redis-master` (sentinel) are running.

**Solution**:
```bash
# Stop sentinel stack
docker compose -f compose/legacy/docker-compose.sentinel.yml down

# Then retry dev stack
make dev-up
```

### Container Won't Start

**Symptom**: Service stuck in "starting" state or exits immediately.

**Debug**:
```bash
# Check logs for specific service
docker logs alebus-simulator
docker logs alebus-ingestor

# Check all dev stack logs
make dev-logs
```

### Healthcheck Failures

**Symptom**: Dependent services won't start because healthcheck fails.

**Debug**:
```bash
# Manual healthcheck for postgres
docker exec alebus-postgres pg_isready -U alebus -d alebus

# Manual healthcheck for redis
docker exec alebus-redis-emqx redis-cli ping

# Manual healthcheck for emqx
docker exec alebus-emqx emqx ping
```

---

## Further Reading

- Root [README.md](../README.md) - Quick start guide
- [simulation_preview/README.md](../simulation_preview/README.md) - Simulation preview setup
- [RUNTIME_AUDIT.md](../RUNTIME_AUDIT.md) - Runtime topology documentation
