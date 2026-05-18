package repositories

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type JourneyRepository interface {
	Save(ctx context.Context, journey *aggregates.Journey) error
	FindByID(ctx context.Context, id types.JourneyID) (*aggregates.Journey, error)
	FindActiveByUserID(ctx context.Context, userID types.UserID) (*aggregates.Journey, error)
	CountActiveByUserID(ctx context.Context, userID types.UserID) (int, error)
}
