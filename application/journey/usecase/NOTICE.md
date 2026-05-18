# journeyusecase — omitted in public release

This package holds the application use cases that orchestrate the journey aggregate in the private build:

- **GPS enrichment pipeline** — `GPSEnrichmentUseCase` converts raw GPS telemetry into enriched live bus state by composing assignment cache + route geometry cache + resolver state + atomic Redis publishing.
- **Live bus ingestion** — `LiveBusIngestion`, `LiveBusHandler` validate and persist live bus updates via the publisher port.
- **Bus recommendations** — `FindBusRecommendationsHandler` composes journey read model + bus read model into ranked candidates.
- **Smart journey plan** — `FindSmartJourneyPlanHandler` decides between single-leg and two-leg plans.
- **Two-leg planner** — `FindTwoLegJourneyRecommendationsHandler` finds transfers across routes via shared stops.
- **Streaming entry points** — `OpenJourneyBusStreamUseCase`, `DeriveWatchedBusIDsUseCase` for SSE/WS endpoints.
- **Push notifications** — `NotifyJourneyProximityUseCase` integrates proximity events with the user notification gate.

The runtime implementations are intentionally not included in this public copy. The ports they consume ([`../ports/`](../ports/)) and the DTOs they emit ([`../dto/`](../dto/), [`../readmodel/`](../readmodel/), [`../trackingreadmodel/`](../trackingreadmodel/)) remain to document the boundary.
