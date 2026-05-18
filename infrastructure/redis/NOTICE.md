# infrastructure/redis — partial public release

This package contains a Redis client and observability foundation. The following pieces are present in the public release:

- `connection.go` — pooled client + `Config`, `Client`, `PreloadScripts` plumbing
- `sentinel_client.go` — Redis Sentinel client wrapper
- `metrics.go`, `observability.go`, `structured_logging.go` — metrics, health, structured logs
- `pubsub.go` — generic pub/sub helpers
- `backup.go` — backup / restore utilities

The following live-bus and resolver state machinery is intentionally omitted from this public copy:

- Live bus state writers, readers, proximity finders (`*_finder.go`, `*_state_reader.go`)
- Atomic Redis Lua scripts and the publisher that orchestrates them (`lua_scripts.go`, `atomic_*.go`)
- Resolver state store (`resolver_state_store.go`)
- Route geometry / bus assignment caches (`route_geometry_cache.go`, `assignment_cache.go`)
- Live-bus and journey pub/sub fan-out hubs (`livebus_updates_hub.go`, `journey_updates_hub.go`)
- Anomaly detection and alerting (`anomaly_detector.go`, `alerting.go`)
- Notification gating (`notification_gate.go`)

These are the components that encode product-specific data layouts and write-path semantics.
