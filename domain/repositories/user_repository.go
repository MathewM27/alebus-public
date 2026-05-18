package repositories

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type UserRepository interface {
	Save(ctx context.Context, user *aggregates.User) error
	FindByID(ctx context.Context, id types.UserID) (*aggregates.User, error)
}