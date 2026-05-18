package busmgmt

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
)

// BusCommandHandler orchestrates bus aggregate lifecycle.
// It does NOT contain business logic - only coordination:
// 1. Load Aggregate (for updates) or Create New
// 2. Call Aggregate methods
// 3. Save via repository

type BusCommandHandler struct {
	repo repositories.BusRepository
}

func NewBusCommandHandler(repo repositories.BusRepository) *BusCommandHandler {
	return &BusCommandHandler{
		repo: repo,
	}
}

func (h *BusCommandHandler) CreateBus(ctx context.Context, cmd CreateBusCommand) error {
	bus, err := aggregates.NewBus(
		cmd.BusID,
		cmd.OperatorID,
		cmd.RouteID,
		cmd.InitalPosition,
		cmd.InitialStopIndex,
		cmd.Direction,
		cmd.CreatedAt,
		cmd.EventRecorder,
	)
	if err != nil {
		return err
	}

	return h.repo.Save(ctx, bus)
}

func (h *BusCommandHandler) UpdateBusPosition(ctx context.Context, cmd UpdateBusPositionCommand) error {
	bus, err := h.repo.FindByID(ctx, cmd.BusID)
	if err != nil {
		return err
	}
	if err := bus.UpdatePosition(cmd.NewPosition, cmd.StopIndex, cmd.SpeedKmh); err != nil {
		return err
	}

	return h.repo.Save(ctx, bus)
}

func (h *BusCommandHandler) ChangeBusDirection(ctx context.Context, cmd ChangeBusDirectionCommand) error {
	bus, err := h.repo.FindByID(ctx, cmd.BusID)
	if err != nil {
		return err
	}
	if err := bus.ChangeDirection(cmd.NewDirection); err != nil {
		return err
	}
	return h.repo.Save(ctx, bus)
}

func (h *BusCommandHandler) ArriveAtTerminal(ctx context.Context, cmd ArriveAtTerminalCommand) error {
	bus, err := h.repo.FindByID(ctx, cmd.BusID)
	if err != nil {
		return err
	}
	if err := bus.ArriveAtTerminal(cmd.ArrivalTime); err != nil {
		return err
	}
	return h.repo.Save(ctx, bus)
}

func (h *BusCommandHandler) DepartFromTerminal(ctx context.Context, cmd DepartFromTerminalCommand) error {
	bus, err := h.repo.FindByID(ctx, cmd.BusID)
	if err != nil {
		return err
	}
	if err := bus.DepartFromTerminal(cmd.DepartureTime); err != nil {
		return err
	}
	return h.repo.Save(ctx, bus)
}

func (h *BusCommandHandler) ChangeBusStatus(ctx context.Context, cmd ChangeBusStatusCommand) error {
	bus, err := h.repo.FindByID(ctx, cmd.BusID)
	if err != nil {
		return err
	}
	if err := bus.ChangeStatus(cmd.Status, cmd.Reason); err != nil {
		return err
	}
	return h.repo.Save(ctx, bus)
}
