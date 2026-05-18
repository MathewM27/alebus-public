package journeymgmt

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
)

// JourneyCommandHandler orchestrates journey aggregate lifecycle.
// It does NOT contain business logic - only coordination:
// 1. Load Aggregate (for updates) or Create New
// 2. Call Aggregate methods
// 3. Save via repository
type JourneyCommandHandler struct {
	repo repositories.JourneyRepository
}

// NewJourneyCommandHandler creates a new JourneyCommandHandler.
func NewJourneyCommandHandler(repo repositories.JourneyRepository) *JourneyCommandHandler {
	return &JourneyCommandHandler{repo: repo}
}

// CreateJourney creates a new journey aggregate and persists it.
func (h *JourneyCommandHandler) CreateJourney(ctx context.Context, cmd CreateJourneyCommand) error {
	journey, err := aggregates.NewJourney(
		cmd.JourneyID,
		cmd.UserID,
		cmd.OriginLocation,
		cmd.OriginStopID,
		cmd.DestinationStopID,
		cmd.CreatedAt,
		cmd.EventRecorder,
	)
	if err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// InitializeTracking loads a journey and initializes tracking with recommendations.
func (h *JourneyCommandHandler) InitializeTracking(ctx context.Context, cmd InitializeTrackingCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.JourneyID)
	if err != nil {
		return err
	}

	if err := journey.InitializeTracking(cmd.Recommendations, cmd.Duration, cmd.RequiredDirection); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// UpdateRecommendations loads a journey and updates its bus recommendations.
func (h *JourneyCommandHandler) UpdateRecommendations(ctx context.Context, cmd UpdateRecommendationsCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.JourneyID)
	if err != nil {
		return err
	}

	if err := journey.UpdateRecommendations(cmd.Recommendations); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// SwitchBus loads a journey and switches to a different bus.
func (h *JourneyCommandHandler) SwitchBus(ctx context.Context, cmd SwitchBusCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.JourneyID)
	if err != nil {
		return err
	}

	if err := journey.SwitchBus(cmd.NewBusID, cmd.Reason); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// UpdateProximity loads a journey and updates bus proximity level.
func (h *JourneyCommandHandler) UpdateProximity(ctx context.Context, cmd UpdateProximityCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.JourneyID)
	if err != nil {
		return err
	}

	if err := journey.UpdateProximity(cmd.BusID, cmd.Level, cmd.Distance); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// ConfirmBoarding loads a journey and confirms the user has boarded.
func (h *JourneyCommandHandler) ConfirmBoarding(ctx context.Context, cmd ConfirmBoardingCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.Journey)
	if err != nil {
		return err
	}

	if err := journey.ConfirmBoarded(); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// DeclineBoarding loads a journey and records that the user declined to board.
func (h *JourneyCommandHandler) DeclineBoarding(ctx context.Context, cmd DeclineBoardingCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.Journey)
	if err != nil {
		return err
	}

	if err := journey.DeclineBoarding(); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// CompleteJourney loads a journey and marks it as completed.
func (h *JourneyCommandHandler) CompleteJourney(ctx context.Context, cmd CompleteJourneyCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.Journey)
	if err != nil {
		return err
	}

	if err := journey.CompleteJourney(); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// CancelJourney loads a journey and cancels it.
func (h *JourneyCommandHandler) CancelJourney(ctx context.Context, cmd CancelJourneyCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.Journey)
	if err != nil {
		return err
	}

	if err := journey.CancelJourney(); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}

// ExpireJourney loads a journey and marks it as expired.
func (h *JourneyCommandHandler) ExpireJourney(ctx context.Context, cmd ExpireJourneyCommand) error {
	journey, err := h.repo.FindByID(ctx, cmd.Journey)
	if err != nil {
		return err
	}

	if err := journey.ExpireJourney(cmd.ExpiredAt); err != nil {
		return err
	}

	return h.repo.Save(ctx, journey)
}
