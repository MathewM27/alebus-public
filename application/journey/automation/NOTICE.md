# journeyautomation — omitted in public release

This package orchestrates the journey lifecycle in the private build:

- `StartJourneyHandler` — discovers feasible routes, applies late-snapping to choose origin stop + direction + bus, and creates a `Journey` aggregate ready for tracking.
- `RefreshJourneyTrackingHandler` — recomputes recommendations and advances proximity for an existing journey.
- `AutoUpdateJourneyTrackingHandler` — background "refresh + switch active bus" tick used by the journey worker.
- `recommendation_builder` — index-first ranking, dedup across overlapping routes, confidence-weighted scoring, ETA + notification level derivation.

The runtime implementations are intentionally not included in this public copy. The ports they consume ([`../ports/`](../ports/)) and the DTOs they emit ([`../dto/`](../dto/), [`../readmodel/`](../readmodel/)) are preserved so the architectural seams remain visible.
