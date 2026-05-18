# Alebus — Real-time Bus Tracking Platform (Public Showcase)

> **This is the public showcase build.** The proprietary algorithms (GPS-to-route projection, recommendation ranking, atomic Redis write semantics, journey lifecycle automation) are intentionally not included. See [What's Included vs Omitted](#whats-included-vs-omitted) below.

Alebus is a real-time public transit tracking platform: passengers see live buses on a map, get recommended boarding stops + buses for their journey, and receive push notifications when their bus is approaching. The backend ingests GPS telemetry from buses, projects each fix onto a route, and pushes enriched live-state to clients over WebSocket/SSE.

This repository documents the **architecture** — domain model, application boundaries, infrastructure plumbing, HTTP/MQTT/Redis layering — rather than ship a runnable product. Reviewers can read the code to understand how the system is structured; they cannot reproduce the live tracking engine from this copy.

---

## What's Included vs Omitted

### Included (build-on-able patterns)

| Layer                            | What's shown                                                                                                       |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| [domain/](domain/)               | Aggregates (Bus, Journey, Route, User), value objects, enums, events, repository interfaces — clean DDD            |
| [application/](application/)     | Use case handlers for bus/route/user/journey management; ports + DTOs for the journey GPS pipeline                 |
| [api/openapi.yaml](api/openapi.yaml) + [api/clients/ts/](api/clients/ts/) | Schema-first contract + generated TypeScript client                                          |
| [api/http/](api/http/)           | Principal/scope auth context, JSON envelopes, SSE connection limiter, WebSocket frame helpers                      |
| [internal/presentation/httpapi/](internal/presentation/httpapi/) | Middleware: auth, CORS, security headers, recovery, request ID, audit logging, metrics, proxy, max body |
| [infrastructure/db/](infrastructure/db/) + [infrastructure/migrations/](infrastructure/migrations/) | Postgres + PostGIS schema migrations                                  |
| [infrastructure/repositories/](infrastructure/repositories/) | Repository implementations (routes, buses, users, journeys, stops, push tokens)                      |
| [infrastructure/mqtt/](infrastructure/mqtt/) | MQTT client, ack manager, heartbeat, bounded queue, metrics, raw-GPS publisher                                |
| [infrastructure/redis/](infrastructure/redis/) | Pooled client, Sentinel client, metrics, observability, pub/sub plumbing, backup utilities                  |
| [infrastructure/push/](infrastructure/push/) | Expo push notification sender                                                                                 |
| [observability/](observability/) | Grafana dashboards + Prometheus alerts                                                                            |
| [admin_ui/](admin_ui/), [observability/static/](observability/static/) | Static admin / observability dashboards                                                  |
| [compose/dev.yml](compose/dev.yml), [Makefile](Makefile), [.golangci.yml](.golangci.yml), [.github/workflows/](.github/workflows/) | Local dev stack + lint/CI configuration                          |

### Omitted (proprietary)

| Component                          | What was there                                                                                                      |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `application/journey/automation/`  | Recommendation ranking algorithm, dedup, start/refresh/auto-update journey orchestration                            |
| `application/journey/usecase/`     | GPS enrichment pipeline, smart plan + two-leg planner, live bus ingestion, journey proximity push                   |
| `application/journey/resolver/`    | Resolver state machine (BOOTSTRAP/TRACKING/REACQUIRE), stop projection, hysteresis, direction inference, terminal detection |
| `application/journey/shared/`      | `ComputeStopIndex` algorithm, confidence-weighted scoring, ETA derivation                                           |
| `application/journey/adapters/`    | Redis bus reader adapters, eligibility filtering, location finders                                                  |
| `infrastructure/redis/` (partial)  | Atomic Lua scripts, live-bus publisher, resolver state store, route geometry cache, anomaly detection, alerting     |
| `cmd/journey_worker/`              | Background worker that ticks `AutoUpdateJourneyTracking` per active journey                                         |
| `cmd/emqx_ingestor/`               | MQTT consumer that runs the GPS enrichment pipeline                                                                 |
| `cmd/gps_simulator/`, `simulation_preview/` | GPS simulator for local development                                                                        |
| Internal audits and design docs    | Implementation traces, mismatch investigations, phase-by-phase scope notes                                          |

Each omitted package directory carries a `NOTICE.md` describing what would be there in the private build.

---

## Architecture at a Glance

```
                  ┌─────────────────────────────────────────┐
   GPS device →   │  MQTT (EMQX) → ingestor → GPS enrich    │
                  │              ↓ resolver state           │
                  │              ↓ atomic Redis write       │
                  │              ↓ pub/sub fan-out          │
                  └─────────────────────────────────────────┘
                                ↓
   Mobile app ←   WebSocket / SSE  ←  api (HTTP) ←  Postgres + Redis read
                                ↓
                  Push notifications (Expo)
```

The proprietary parts are the boxes marked "GPS enrich" and "atomic Redis write" — everything else is in this repo as patterns and plumbing.

---

## Local Development

> The public copy does **not** build or run as a complete application — too many of the runtime wiring components are intentionally omitted. You can still:
>
> - Inspect the domain model and DDD layering
> - Read the HTTP middleware stack and how it's composed
> - Read the migrations and Postgres/Redis client setup
> - Read the MQTT client + ack manager + heartbeat code
> - Compile individual `application/{bus,route,user}/mgmt/` and `domain/` packages
>
> For a runnable build, see the private repository.

```bash
# Bring up Postgres + Redis + EMQX
docker compose -f compose/dev.yml up -d

# Run migrations against the local Postgres
make migrate-up

# Inspect the OpenAPI spec
make api-openapi-lint
```

---

## Repository Layout

```
domain/                          DDD aggregates, value objects, enums, events
application/
├─ bus/      mgmt, ports, readmodel, streaming, usecase    (command + query)
├─ route/    mgmt                                          (command)
├─ user/     mgmt, ports, readmodel, usecase               (command + push)
└─ journey/  mgmt, ports, dto, readmodel, …                (PUBLIC: contracts only)
api/
├─ http/      sse, ws_common, json, principal              (transport helpers)
├─ openapi.yaml + clients/ts/                              (API contract)
internal/
├─ presentation/httpapi/ middleware/* (12 middleware files)
└─ wire/    config.go, postgres.go                         (DI plumbing skeleton)
infrastructure/
├─ db/       pooled pgx connection
├─ migrations/  PostgreSQL + PostGIS migrations
├─ mqtt/    client, ack manager, heartbeat, queue, metrics
├─ redis/   client, sentinel, metrics, pubsub, observability   (PUBLIC: plumbing only)
├─ repositories/ Postgres-backed aggregate + read-model impls
└─ push/    Expo push sender
observability/  Grafana dashboards + Prometheus alerts
admin_ui/       Static admin UI
```

---

## License

MIT — see [LICENSE](LICENSE).
