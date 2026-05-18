package routemgmt

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
)

// RouteCommandHandler orchestrates route aggregate lifecycle.
// It does NOT contain business logic - only coordination:
// 1. Load Aggregate (for updates) or Create New
// 2. Call Aggregate methods
// 3. Save via repository
type RouteCommandHandler struct {
	repo repositories.RouteRepository
}

// NewRouteCommandHandler creates a new RouteCommandHandler.
func NewRouteCommandHandler(repo repositories.RouteRepository) *RouteCommandHandler {
	return &RouteCommandHandler{repo: repo}
}

// CreateRoute creates a new route aggregate and persists it.
// Note: NewRoute() automatically calculates cumulative distances for all stops
// to enable segment-based distance computation per algorithm.md.
func (h *RouteCommandHandler) CreateRoute(ctx context.Context, cmd CreateRouteCommand) error {
	route, err := aggregates.NewRoute(
		cmd.RouteID,
		cmd.OperatorIDs,
		cmd.Stops,
		cmd.Name,
		cmd.Direction,
		cmd.RouteType,
		cmd.ActiveFrom,
		cmd.ActiveUntil,
		cmd.CreatedAt,
		cmd.EventRecorder,
	)
	if err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}

// UpdateRouteStops loads a route and updates its stops.
func (h *RouteCommandHandler) UpdateRouteStops(ctx context.Context, cmd UpdateRouteStopsCommand) error {
	route, err := h.repo.FindByID(ctx, cmd.RouteID)
	if err != nil {
		return err
	}

	if err := route.UpdateStops(cmd.Stops, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}

// ChangeRouteStatus loads a route and changes its status.
func (h *RouteCommandHandler) ChangeRouteStatus(ctx context.Context, cmd ChangeRouteStatusCommand) error {
	route, err := h.repo.FindByID(ctx, cmd.RouteID)
	if err != nil {
		return err
	}

	if err := route.ChangeStatus(cmd.Status, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}

// ChangeRouteName loads a route and changes its name.
func (h *RouteCommandHandler) ChangeRouteName(ctx context.Context, cmd ChangeRouteNameCommand) error {
	route, err := h.repo.FindByID(ctx, cmd.RouteID)
	if err != nil {
		return err
	}

	if err := route.ChangeName(cmd.Name, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}

// ChangeRouteType loads a route and changes its type.
func (h *RouteCommandHandler) ChangeRouteType(ctx context.Context, cmd ChangeRouteTypeCommand) error {
	route, err := h.repo.FindByID(ctx, cmd.RouteID)
	if err != nil {
		return err
	}

	if err := route.ChangeRouteType(cmd.RouteType, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}

// ChangeActivePeriod loads a route and changes its active period.
func (h *RouteCommandHandler) ChangeActivePeriod(ctx context.Context, cmd ChangeActivePeriodCommand) error {
	route, err := h.repo.FindByID(ctx, cmd.RouteID)
	if err != nil {
		return err
	}

	if err := route.ChangeActivePeriod(cmd.ActiveFrom, cmd.ActiveUntil, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}

// AddStop loads a route and adds a stop at the specified position.
func (h *RouteCommandHandler) AddStop(ctx context.Context, cmd AddStopCommand) error {
	route, err := h.repo.FindByID(ctx, cmd.RouteID)
	if err != nil {
		return err
	}

	if err := route.AddStop(cmd.Stop, cmd.Position, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}

// RemoveStop loads a route and removes a stop by ID.
func (h *RouteCommandHandler) RemoveStop(ctx context.Context, cmd RemoveStopCommand) error {
	route, err := h.repo.FindByID(ctx, cmd.RouteID)
	if err != nil {
		return err
	}

	if err := route.RemoveStop(cmd.StopID, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, route)
}
