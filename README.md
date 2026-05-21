# Alebus

**Real-time public transport intelligence platform for Mauritius**

GPS telemetry from bus-mounted devices flows through an MQTT pipeline into an event-driven Go backend, fans out via Redis Pub/Sub to WebSocket clients, and surfaces as live bus locations, stop-by-stop ETAs, and fleet analytics — in under 760ms end-to-end.

![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=flat&logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-DC382D?style=flat&logo=redis&logoColor=white)
![MQTT](https://img.shields.io/badge/MQTT-660066?style=flat&logo=mqtt&logoColor=white)
![EMQX](https://img.shields.io/badge/EMQX-39F0D8?style=flat&logo=emqx&logoColor=black)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat&logo=docker&logoColor=white)
![React Native](https://img.shields.io/badge/React_Native-20232A?style=flat&logo=react&logoColor=61DAFB)
![Next.js](https://img.shields.io/badge/Next.js-000000?style=flat&logo=next.js&logoColor=white)
![Prometheus](https://img.shields.io/badge/Prometheus-E6522C?style=flat&logo=prometheus&logoColor=white)
![Grafana](https://img.shields.io/badge/Grafana-F2CC0C?style=flat&logo=grafana&logoColor=white)

> **License:** Creative Commons Attribution-NonCommercial 4.0 International (CC BY-NC 4.0). Attribution to Mathews Mwangi required. Commercial use prohibited. See [LICENSE](./LICENSE).

> **📌 About this repository — public showcase build.** This README documents the **full production system** as designed, built, and deployed. The runnable binaries (`cmd/api`, `cmd/emqx_ingestor`, `cmd/emqx_auth`, `cmd/journey_worker`, `cmd/gps_simulator`) and the proprietary algorithm packages (GPS-to-route resolver, recommendation ranking, atomic Redis Lua write semantics, journey lifecycle automation) are **intentionally omitted from this public copy** to protect the working implementation. What ships here is the architectural skeleton — domain model, application boundaries, HTTP middleware, MQTT plumbing, Redis client infrastructure, migrations, observability configuration, and OpenAPI contract. Each omitted package directory carries a `NOTICE.md` describing what would be there in the private build.

---

## Preview

### Commuter App

| Smart Search | Live Journey | Profile |
|:---:|:---:|:---:|
| ![Smart Search](docs/screenshots/app-smart-search.png) | ![Live Journey](docs/screenshots/app-journey.png) | ![Profile](docs/screenshots/app-profile.png) |
| Input origin and destination — system matches buses heading the right direction that haven't passed your stop | Real-time bus position, stop-by-stop ETAs, distance to arrival | Theme, credentials, subscription management |

### Operator Dashboard

![Operator Dashboard Overview](docs/screenshots/dashboard-overview.png)

| Fleet Map | Route Performance | Driver Behaviour |
|:---:|:---:|:---:|
| ![Fleet Map](docs/screenshots/dashboard-fleet.png) | ![Route Performance](docs/screenshots/dashboard-routes.png) | ![Driver Behaviour](docs/screenshots/dashboard-drivers.png) |

---

## Motivation

Mauritius has no public real-time bus tracking. Commuters wait at stops with no visibility of when — or whether — a bus is coming; waits regularly exceed 45 minutes with no way to tell if the next bus is 2 minutes or 45 minutes away. Operators have no live view of their fleet — no live map, no route adherence, no delay detection.

Alebus was built to solve both sides of that problem: give commuters a live, direction-aware view of the buses approaching their stop, and give operators a real-time picture of their fleet as it moves. See [Background](#background) for the full project history.

---

## What It Does

**For commuters:** Open the app, enter where you are and where you're going. The system finds buses on matching routes that are heading toward your destination and haven't yet passed your boarding stop — accounting for direction, route assignment, and real-time position. A live map shows the bus moving toward you with ETAs that update every few seconds.

**For operators:** A web dashboard shows every bus in the fleet on a live map, with route adherence monitoring, driver behaviour analytics, delay detection, and speeding alerts. No manual check-ins. No phone calls. Live visibility as the fleet moves.

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Bus GPS Devices                            │
│              (Android app, pinging every 4 seconds)            │
└───────────────────────────┬─────────────────────────────────────┘
                            │  MQTT over TLS
                            ▼
                 ┌─────────────────────┐
                 │     EMQX Broker     │◄────── cmd/emqx_auth
                 │   (MQTT broker)     │        HTTP authn + ACL hooks
                 └──────────┬──────────┘        per-device topic isolation
                            │
                            ▼
                 ┌─────────────────────┐
                 │  cmd/emqx_ingestor  │  Ingestion worker
                 │    (Go binary)      │  Parses GPS payloads
                 └──────────┬──────────┘  Publishes to Redis
                            │
              ┌─────────────▼──────────────┐
              │           Redis             │
              │   Pub/Sub + live state      │  Bus positions, direction,
              │   (current positions only)  │  route assignment, heading
              └──────┬─────────────┬────────┘
                     │             │
                     ▼             ▼
          ┌──────────────┐  ┌──────────────────┐
          │  cmd/api     │  │ cmd/journey_worker│
          │  HTTP API    │  │ Background jobs   │
          │  WebSocket   │  │ Journey state     │
          │  fan-out     │  │ processing        │
          └──────┬───────┘  └──────────────────┘
                 │
     ┌───────────▼────────────┐
     │                        │
┌────▼──────┐          ┌──────▼──────┐
│ Commuter  │          │  Operator   │
│ React     │          │  Next.js    │
│ Native    │          │  Dashboard  │
│ App       │          │             │
└───────────┘          └─────────────┘
```

### Data flow in detail

1. A bus-mounted Android device pings its GPS coordinates every 4 seconds via MQTT to the EMQX broker
2. EMQX validates the device credentials and topic permissions via `cmd/emqx_auth` before accepting the message
3. `cmd/emqx_ingestor` receives the payload, parses coordinates and metadata, and publishes the update to a Redis Pub/Sub channel
4. `cmd/api` subscribes to those channels and fans out the update to all connected WebSocket clients interested in that bus
5. The commuter app and operator dashboard receive the update and re-render — typically within 760ms of the original GPS ping
6. `cmd/journey_worker` runs independently, processing route state, detecting delays, and updating journey read models in PostgreSQL

---

## Domain-Driven Design and Hexagonal Architecture

This is the most deliberate architectural decision in the codebase. The backend is structured as a Go monolith with hexagonal (ports and adapters) architecture and a full DDD domain model. The goal was a system where business logic has zero awareness of infrastructure — no database imports in the domain, no Redis calls in use cases, no HTTP concepts leaking into the application layer.

### Why DDD for a transport platform

Public transport is a domain with genuine complexity. A bus is not just a GPS coordinate — it has a route assignment, a direction, a driver, a schedule, a list of upcoming stops, and a state machine governing whether it's in service, delayed, or off-route. A journey is not just "user goes from A to B" — it involves matching the user's boarding stop to buses that are approaching (not passed), on the right route, heading the correct direction.

Without a proper domain model, this logic ends up scattered across HTTP handlers and database queries. With DDD, the rules live in the domain layer where they can be reasoned about, tested, and evolved without touching infrastructure.

### The four bounded contexts

The application layer is divided into four bounded contexts, each owning its own use cases and read models:

```
application/
├── journey/       GPS tracking automation, route matching,
│                  boarding logic, streaming, journey read models
│
├── bus/           Bus entity management, live state streaming,
│                  real-time position read models
│
├── route/         Route definitions, stop management,
│                  route-stop relationships, geospatial data
│
└── user/          User accounts, authentication use cases,
                   subscription management, preferences
```

Each bounded context communicates with others through domain events and repository interfaces — never through direct imports of another context's internals. This means one context cannot accidentally query another context's database tables; it can only receive events that the other context explicitly publishes.

### Hexagonal architecture layers

```
┌─────────────────────────────────────────────────────────┐
│                    Presentation Layer                    │
│         HTTP handlers · WebSocket server · Middleware    │
│              internal/presentation/httpapi/              │
└───────────────────────────┬─────────────────────────────┘
                            │ calls
┌───────────────────────────▼─────────────────────────────┐
│                   Application Layer                      │
│     Use cases · Ports (interfaces) · Bounded contexts    │
│                    application/                          │
└──────┬──────────────────────────────────────┬────────────┘
       │ uses domain types                    │ calls via interfaces
┌──────▼──────────┐                ┌──────────▼────────────┐
│  Domain Layer   │                │ Infrastructure Layer   │
│  Pure business  │                │ Postgres · Redis       │
│  logic, no      │                │ MQTT · Migrations      │
│  dependencies   │                │ infrastructure/        │
│  domain/        │                └───────────────────────┘
└─────────────────┘
```

**Domain layer** (`domain/`) has zero external dependencies. No database drivers, no Redis clients, no HTTP packages. It contains aggregates, value objects, domain events, enums, and repository interfaces. This layer defines what the system *is*, not how it works.

**Application layer** (`application/`) contains use cases that orchestrate domain objects. Use cases depend on repository interfaces (ports), never on concrete implementations. This is what makes the system testable without a running database.

**Infrastructure layer** (`infrastructure/`) contains the adapters — PostgreSQL repository implementations, Redis Pub/Sub clients, MQTT subscribers, database migration scripts. These are the only places where external dependencies appear.

**Presentation layer** (`internal/presentation/httpapi/`) contains HTTP handlers and WebSocket management. Handlers call use cases; they do not contain business logic.

**Wire layer** (`internal/wire/`) wires everything together at startup — injecting concrete infrastructure implementations into use cases, use cases into handlers. Dependency injection is manual (no framework), which keeps the startup flow explicit and debuggable.

### Domain model example: journey matching

The smart search feature — matching a commuter to buses heading toward their destination — is a good example of domain logic that would be a mess without a proper model.

The naive implementation would be: query buses near the commuter, return the closest. This breaks immediately because "closest" is meaningless without direction — a bus 200 metres away heading away from you is less useful than one 800 metres away heading toward you.

The domain model handles this through `application/journey/`:

- Each bus has a `Direction` field (outbound/inbound) derived from its route assignment, stored in Redis
- `IsBeforeBoarding()` and `IsBeforeBoardingByCumulativeDistance()` compare the bus's current stop index against the commuter's boarding stop index along the route
- The recommendation builder categorises buses as: **primary** (same direction, not yet passed the boarding stop), **opposite** (other direction — will arrive after looping), or **passed** (already went by)
- PostGIS handles stop proximity (`ST_DWithin` with GIST index) — finding which stops are near the commuter's location
- Direction-awareness is entirely application-layer logic — PostGIS does geometry, Go does the route reasoning

This separation means the direction logic can be unit tested without PostGIS, and the PostGIS queries can be tuned without touching business rules.

---

## Key Engineering Decisions

### Go standard library over a framework

The HTTP surface area — a handful of REST endpoints and a WebSocket upgrade handler — doesn't justify a framework. `net/http` handles it cleanly. The interesting complexity in this system lives in the domain and real-time layers, not in routing. Keeping the dependency surface minimal also means zero risk of framework-specific bugs or upgrade breakage at the transport layer.

### MQTT over HTTP polling for GPS ingestion

Bus-mounted devices operate on mobile data connections with variable signal quality. MQTT's persistent connection and lightweight binary framing are designed for exactly this environment — lower bandwidth consumption, built-in reconnection handling, and QoS guarantees that HTTP polling can't provide. At 4-second ping intervals across a fleet, the bandwidth difference compounds meaningfully.

### Redis Pub/Sub over Kafka for fan-out

The real-time position update use case has one requirement: deliver the current position to all connected clients as fast as possible. It does not require message replay, consumer groups, or retention — once a bus has moved, the previous position is irrelevant. Kafka's value proposition is built around those features. At this scale, Redis Pub/Sub provides lower latency with zero operational overhead. The architecture is designed so that a persistent message queue (Kafka or similar) could be introduced at the ingestion layer if audit trails or replay became requirements.

### PostgreSQL for structured data, Redis for live state

PostgreSQL stores everything that needs to be queried, reported on, or audited — routes, stops, journeys, operator analytics, user accounts. Redis stores only what needs to be read at sub-millisecond speed per WebSocket tick — current bus positions, directions, route assignments. The split is deliberate: Redis is not used as a database of record, only as a live state cache. If Redis were wiped, the system would lose real-time positions temporarily but no permanent data.

### PostGIS for spatial proximity, application logic for direction

PostGIS handles the geometry problem — "which stops are within 300 metres of this GPS coordinate" — because it does this extremely well with `ST_DWithin` and GIST spatial indexes. It does not handle the route-reasoning problem — "is this bus approaching or has it passed this stop" — because that logic requires knowledge of route structure, stop ordering, and bus direction that lives more naturally in Go application code. Mixing spatial queries with route reasoning in SQL would produce unmaintainable queries. Keeping them separated means each piece is independently testable and understandable.

### Hexagonal architecture for infrastructure independence

Designing around ports and adapters meant the GPS input layer could be designed from day one to be replaceable — the system was initially built around Android devices with a custom GPS app but architected so Teltonika FMC hardware could drop in as a replacement without touching business logic. Any technology change at the infrastructure boundary (swap Postgres for CockroachDB, swap EMQX for HiveMQ) requires only a new adapter, not a domain rewrite.

---

## Technical Highlights

| Metric | Value |
|---|---|
| End-to-end latency (GPS ping → client render) | ~760ms median |
| Concurrent WebSocket connections (load tested) | 10,000 per instance |
| WebSocket scalability | Horizontal via Redis Pub/Sub |
| GPS ping frequency | Every 4 seconds per device |
| Real-world testing | ~1 month on live Mauritius bus routes |
| Architecture | DDD · Hexagonal · 4 bounded contexts |
| Runnable binaries | 5 (sharing one Go module) |
| Observability | Prometheus · Grafana · Dozzle |
| Deployment | Dockerised · GitHub Actions CI/CD · Hetzner VPS |

---

## Project Structure

```
alebus-api/
│
├── cmd/                        Runnable binaries (omitted from public copy)
│   ├── api/                    Main HTTP API + WebSocket server
│   ├── emqx_ingestor/          MQTT GPS ingestion worker
│   ├── emqx_auth/              EMQX HTTP authn + authz hook
│   ├── journey_worker/         Background journey state processor
│   └── gps_simulator/          Simulated GPS fleet for testing
│
├── domain/                     Pure business logic — zero external deps
│   ├── aggregates/             Bus, Journey, Route, User aggregates
│   ├── valueobjects/           Coordinate, StopIndex, Direction, ETA
│   ├── enums/                  BusStatus, JourneyState, Direction
│   ├── events/                 Domain events (BusMoved, JourneyStarted)
│   ├── types/                  Shared domain types
│   └── repositories/           Repository interfaces (ports only)
│
├── application/                Use cases, one package per bounded context
│   ├── journey/                Route matching, boarding logic, ETAs
│   ├── bus/                    Bus state management, live streaming
│   ├── route/                  Route and stop management
│   └── user/                   Auth, accounts, subscriptions
│
├── infrastructure/             Adapters implementing domain ports
│   ├── repositories/           PostgreSQL implementations
│   ├── redis/                  Redis Pub/Sub, Lua scripts (partial in public)
│   ├── mqtt/                   EMQX integration layer
│   ├── push/                   Push notification adapters
│   ├── db/                     Database connection management
│   └── migrations/             Schema migrations (no seeded data)
│
├── internal/
│   ├── presentation/
│   │   └── httpapi/            HTTP handlers, middleware, WebSocket
│   └── wire/                   Dependency injection wiring
│
├── compose/dev.yml             Local development infrastructure
└── Makefile                    Build, test, and dev commands
```

---

## Runnable Binaries

> The binary implementations are omitted from this public copy. The descriptions below document the role each binary plays in the production system.

All binaries share the same Go module and domain layer. Each has a single, well-defined responsibility.

### `cmd/api` — Main API server

The primary runtime. Serves the REST API for the commuter app and operator dashboard, manages WebSocket connections for real-time updates, and coordinates fan-out via Redis Pub/Sub subscriptions. Stateless — multiple instances can run behind a load balancer.

### `cmd/emqx_ingestor` — GPS ingestion worker

Subscribes to the MQTT broker and processes incoming GPS payloads from all bus devices. Parses coordinates, heading, speed, and device metadata. Publishes normalised position updates to Redis Pub/Sub. Designed to run as a separate process so ingestion throughput scales independently of API serving.

### `cmd/emqx_auth` — MQTT authentication and authorisation

An HTTP callback service implementing EMQX's authentication and ACL hooks. Two endpoints:

- `/auth` — Verifies bus device credentials (username + bcrypt-hashed password) against the `bus_devices` table in PostgreSQL. Handles superuser bypass paths for the ingestor service and GPS simulator using constant-time comparison.
- `/authz` — Enforces per-device topic isolation. Each bus device may only publish to `bus/{its-own-busId}/gps`. The ingestor superuser can publish to any `bus/*/gps` topic. This prevents a compromised device from publishing false positions for other buses.

### `cmd/journey_worker` — Background journey processor

Handles journey state transitions that are too expensive to run synchronously in the API path. Processes route adherence calculations, updates journey read models, detects delays and events, and writes operator analytics to PostgreSQL.

### `cmd/gps_simulator` — Fleet simulator

Generates realistic GPS payloads for a configurable number of simulated buses moving along real routes. Used for load testing and local development. Supports up to 20 concurrent simulated buses. The system was load-tested using this binary alongside `k6` for WebSocket client simulation.

---

## Observability

The full production stack includes three observability layers:

**Prometheus** collects metrics from the API, ingestor, and journey worker — request rates, WebSocket connection counts, Redis Pub/Sub lag, GPS message throughput, queue depths, and error rates.

**Grafana** provides dashboards for all collected metrics. Separate dashboards for real-time system health (latency, connection counts, message rates) and operational overview (active journeys, buses in service, route adherence).

**Dozzle** provides a real-time container log viewer for the full Docker Compose stack — API, ingestor, Redis, PostgreSQL, Nginx — accessible without SSH during active monitoring sessions.

Structured logging throughout the codebase (JSON format in production) means logs are filterable by bus ID, journey ID, device ID, or error type without grep archaeology.

---

## Quick Start

> The public copy of this repository contains the architectural skeleton — domain model, application contracts, HTTP middleware, MQTT plumbing, Redis client infrastructure, and PostgreSQL migrations — but **not the runnable `cmd/*` binaries**. The steps below describe how the full system runs in the private repository.

### Prerequisites

- Go 1.22+
- Docker and Docker Compose v2+
- Make (optional, for convenience commands)
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI (for database migrations)

### Start local infrastructure

```bash
# Spin up Postgres (PostGIS), Redis, and EMQX
docker compose -f compose/dev.yml up -d

# Run migrations
export DATABASE_URL="postgres://alebus:alebus@localhost:5432/alebus?sslmode=disable"
make migrate-up
```

### Run the services (full build)

```bash
# Terminal 1 — EMQX auth hook (must start before ingestor)
go run cmd/emqx_auth/main.go

# Terminal 2 — GPS ingestor
go run cmd/emqx_ingestor/main.go

# Terminal 3 — Main API
go run cmd/api/main.go

# Terminal 4 — Simulate GPS devices
go run cmd/gps_simulator/main.go --buses=5 --routes=2
```

> **Note:** Route and stop data for Mauritius are not included in this repository. The system will start and the API will respond, but journey matching and stop proximity features require a seeded database with real route geometry. This is intentional.

---

## Usage

The running system is exercised through three surfaces — a REST API, real-time WebSocket streams, and the client apps. The OpenAPI contract and TypeScript client referenced below ship in this public copy and are usable as-is; the live API surface requires the full build (see [Quick Start](#quick-start)).

### REST API

The HTTP API is defined by an OpenAPI 3 contract at [`api/openapi.yaml`](api/openapi.yaml) — the single source of truth for every endpoint, request, and response shape. Lint it locally with:

```bash
make api-openapi-lint
```

A generated TypeScript client lives in [`api/clients/ts`](api/clients/ts):

```bash
make api-client-ts-install
make api-client-ts-build
```

### Real-time WebSocket streams

Live bus positions and journey updates are pushed over WebSocket — clients never poll. The API exposes:

| Endpoint | Purpose |
|---|---|
| `/api/v1/ws/mux` | Multiplexed stream (recommended) — many topics over one connection |
| `/api/v1/ws/live-buses` | Live bus positions |
| `/api/v1/ws/journeys` | Journey-specific updates |

### Simulated fleet

Real Mauritius route data is not bundled here, so the GPS simulator (`cmd/gps_simulator`) is how the system is driven in development. It generates realistic GPS payloads for buses moving along routes and publishes them through the same MQTT → ingestor → Redis → WebSocket path as real devices — load sample routes, create buses, start a simulation, and watch updates fan out to connected clients.

### Client applications

The commuter app and operator dashboard are the primary end-user interfaces — see [Related Repositories](#related-repositories).

---

## Related Repositories

| Repository | Description |
|---|---|
| [`alebus-mobile`](https://github.com/MathewM27/alebus-mobile) | React Native commuter app — smart search, live journey map, ETAs |
| [`alebus-dashboard`](https://github.com/MathewM27/alebus-dashboard) | Next.js operator fleet dashboard — live tracking, analytics, driver behaviour |

---

## Background

Alebus was built out of direct frustration with Mauritius's public transport system. Commuters had zero real-time visibility of bus locations — waits regularly exceeded 45 minutes with no way to know if the next bus was 2 minutes or 45 minutes away. Operators had no live view of their fleet.

The platform was designed, built, and deployed to production by a single engineer across 2025–2026. It was validated through a month of real-world road testing on live Mauritius bus routes, and commercial pilot discussions were pursued with the country's largest bus operator and the national Ministry of Transport.

The project was wound down following an unsuccessful public tender. The codebase represents a complete, production-validated implementation — not a prototype or proof of concept.

---

## Contributing

Alebus is a public **showcase build** of a project that has been wound down (see [Background](#background)) — it is not under active development, and the runnable core is intentionally omitted from this repository. External pull requests are therefore not being accepted.

Questions, corrections, and discussion are welcome:

- **Spotted an error** in the documentation or architecture write-up? Open an issue.
- **Curious about the design**, the omitted internals, or the engineering trade-offs? Reach out via [mathewsmwangi.com](https://mathewsmwangi.com) or [LinkedIn](https://linkedin.com/in/mathews-mwangi-972839219).

All use of this code is governed by the [CC BY-NC 4.0 license](./LICENSE): attribution to Mathews Mwangi is required, and commercial use is prohibited.

---

## Architecture Diagram

> 📌 *Full system diagram available in [`docs/architecture.png`](docs/architecture.png)*

---

*Built by [Mathews Mwangi](https://mathewsmwangi.com) · [LinkedIn](https://linkedin.com/in/mathews-mwangi-972839219)*
