package usermgmt

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

// UserCommandHandler orchestrates user aggregate lifecycle.
// It does NOT contain business logic - only coordination:
// 1. Load Aggregate (for updates) or Create New
// 2. Call Aggregate methods
// 3. Save via repository

type UserCommandHandler struct {
	repo repositories.UserRepository
}

func NewUserCommandHandler(repo repositories.UserRepository) *UserCommandHandler {
	return &UserCommandHandler{repo: repo}
}

func (h *UserCommandHandler) CreateUser(ctx context.Context, cmd CreateUserCommand) error {
	user, err := aggregates.NewUser(
		cmd.UserID,
		cmd.Email,
		cmd.Subscription,
		cmd.CreatedAt,
		cmd.EventRecorder,
	)
	if err != nil {
		return err
	}

	return h.repo.Save(ctx, user)
}

func (h *UserCommandHandler) ChangeEmail(ctx context.Context, cmd ChangeEmailCommand) error {
	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}

	newEmail, err := valueobjects.NewEmail(cmd.NewEmail)
	if err != nil {
		return err
	}
	if err := user.ChangeEmail(newEmail); err != nil {
		return err
	}
	return h.repo.Save(ctx, user)
}

func (h *UserCommandHandler) UpdateSubscription(ctx context.Context, cmd UpdateSubscriptionCommand) error {
	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if err := user.UpdateSubscription(cmd.NewSubscription); err != nil {
		return err
	}
	return h.repo.Save(ctx, user)
}

func (h *UserCommandHandler) AddSavedLocation(ctx context.Context, cmd AddSavedLocationCommand) error {
	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}

	location := valueobjects.SavedLocation{
		Name:     cmd.Name,
		Location: types.GeoLocation{Latitude: cmd.Lat, Longitude: cmd.Lon},
		StopID:   cmd.StopID,
	}
	if err := user.AddSavedLocation(location); err != nil {
		return err
	}
	return h.repo.Save(ctx, user)
}

func (h *UserCommandHandler) RemoveSavedLocation(ctx context.Context, cmd RemoveSavedLocationCommand) error {
	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if err := user.RemoveSavedLocation(cmd.Name); err != nil {
		return err
	}
	return h.repo.Save(ctx, user)
}

func (h *UserCommandHandler) UpdateSavedLocation(ctx context.Context, cmd UpdateSavedLocationCommand) error {
	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}

	updated := valueobjects.SavedLocation{
		Name:     cmd.Name,
		Location: types.GeoLocation{Latitude: cmd.Lat, Longitude: cmd.Lon},
		StopID:   cmd.StopID,
	}
	if err := user.UpdateSavedLocation(updated); err != nil {
		return err
	}
	return h.repo.Save(ctx, user)
}

func (h *UserCommandHandler) ClearSavedLocations(ctx context.Context, cmd ClearSavedLocationsCommand) error {
	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}

	if err := user.ClearSavedLocations(); err != nil {
		return err
	}
	return h.repo.Save(ctx, user)
}
