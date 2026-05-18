package journeymgmt

import (
	"context"
	"fmt"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

// SwitchBusByLocationCommand is the input for switching the active bus based on user location.
type SwitchBusByLocationCommand struct {
	JourneyID    types.JourneyID
	UserLocation types.GeoLocation
	Reason       enums.JourneySwitchReason
}

// SwitchBusByLocationResponse is the output after switching the bus.
type SwitchBusByLocationResponse struct {
	JourneyID   string `json:"journeyId"`
	OldBusID    string `json:"oldBusId"`
	NewBusID    string `json:"newBusId"`
	WasSwitched bool   `json:"wasSwitched"`
}

// BusLocationFinder is a port for finding buses near a location.
// This abstracts the geo-lookup previously done inside the Journey aggregate.
// Implementation can use Redis GEO, PostGIS, or other spatial services.
type BusLocationFinder interface {
	// FindBusesNearLocation returns bus IDs near the given location within radiusKm.
	// The busIDs parameter filters results to only include buses from that list.
	FindBusesNearLocation(
		ctx context.Context,
		location types.GeoLocation,
		radiusKm float64,
		busIDs []types.BusID,
	) ([]types.BusLocationInfo, error)
}

// SwitchBusByLocationHandler performs the "switch bus by user location" workflow
// at the application layer, keeping the aggregate pure (no IO).
//
// This replaces the old aggregate-level approach that performed an external geo lookup
// inside the domain (violation of DDD / Clean Architecture principles).
//
// Application responsibilities:
//   - Load journey aggregate
//   - Perform geo lookup via BusLocationFinder port
//   - Call Journey.SwitchBus(newBusID, reason) (pure domain method)
//   - Save journey aggregate
//
// Domain responsibilities:
//   - Enforce business rules for bus switching (expiration, boarding state, etc.)
type SwitchBusByLocationHandler struct {
	journeyRepo       repositories.JourneyRepository
	busLocationFinder BusLocationFinder
}

// NewSwitchBusByLocationHandler creates a new handler with required dependencies.
func NewSwitchBusByLocationHandler(
	journeyRepo repositories.JourneyRepository,
	busLocationFinder BusLocationFinder,
) *SwitchBusByLocationHandler {
	return &SwitchBusByLocationHandler{
		journeyRepo:       journeyRepo,
		busLocationFinder: busLocationFinder,
	}
}

// Handle executes the switch bus by location workflow.
func (h *SwitchBusByLocationHandler) Handle(ctx context.Context, cmd SwitchBusByLocationCommand) (SwitchBusByLocationResponse, error) {
	// ⚠️ EXPLICIT: SwitchBusByLocation REQUIRES Redis
	// This check prevents silent partial behavior and makes the dependency intentional.
	if h.busLocationFinder == nil {
		return SwitchBusByLocationResponse{}, fmt.Errorf("live bus switching requires Redis to be enabled")
	}

	if cmd.JourneyID == "" {
		return SwitchBusByLocationResponse{}, fmt.Errorf("journeyId required")
	}

	// 1) Load journey aggregate
	journey, err := h.journeyRepo.FindByID(ctx, cmd.JourneyID)
	if err != nil {
		return SwitchBusByLocationResponse{}, fmt.Errorf("failed to load journey: %w", err)
	}
	if journey == nil {
		return SwitchBusByLocationResponse{}, fmt.Errorf("journey not found: %s", cmd.JourneyID)
	}

	// Capture old bus ID for response
	oldBusID := journey.ActiveBusID()

	// Get the list of recommended bus IDs to filter geo search
	recommendedBuses := journey.RecommendedBuses()
	if len(recommendedBuses) == 0 {
		return SwitchBusByLocationResponse{
			JourneyID:   string(cmd.JourneyID),
			OldBusID:    string(oldBusID),
			NewBusID:    string(oldBusID),
			WasSwitched: false,
		}, nil
	}

	busIDs := make([]types.BusID, len(recommendedBuses))
	for i, rec := range recommendedBuses {
		busIDs[i] = rec.BusID
	}

	// 2) Perform geo lookup via port (application orchestration, not domain)
	nearbyBuses, err := h.busLocationFinder.FindBusesNearLocation(ctx, cmd.UserLocation, 2.0, busIDs)
	if err != nil || len(nearbyBuses) == 0 {
		// No nearby buses found - keep current bus
		return SwitchBusByLocationResponse{
			JourneyID:   string(cmd.JourneyID),
			OldBusID:    string(oldBusID),
			NewBusID:    string(oldBusID),
			WasSwitched: false,
		}, nil
	}

	newBusID := nearbyBuses[0].BusID
	if newBusID == oldBusID {
		// Already on the nearest bus
		return SwitchBusByLocationResponse{
			JourneyID:   string(cmd.JourneyID),
			OldBusID:    string(oldBusID),
			NewBusID:    string(oldBusID),
			WasSwitched: false,
		}, nil
	}

	// 3) Call the pure domain method to switch bus
	if err := journey.SwitchBus(newBusID, cmd.Reason); err != nil {
		return SwitchBusByLocationResponse{}, fmt.Errorf("failed to switch bus: %w", err)
	}

	// 4) Save the journey aggregate
	if err := h.journeyRepo.Save(ctx, journey); err != nil {
		return SwitchBusByLocationResponse{}, fmt.Errorf("failed to save journey: %w", err)
	}

	return SwitchBusByLocationResponse{
		JourneyID:   string(cmd.JourneyID),
		OldBusID:    string(oldBusID),
		NewBusID:    string(newBusID),
		WasSwitched: true,
	}, nil
}
