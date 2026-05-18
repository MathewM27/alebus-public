package userports

import (
	"context"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type PushPlatform string

const (
	PushPlatformExpo PushPlatform = "expo"
)

type UserPushTokenRecord struct {
	UserID     types.UserID
	DeviceID   string
	Platform   PushPlatform
	Token      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastSeenAt *time.Time
	RevokedAt  *time.Time
}

type UserPushTokenStore interface {
	UpsertToken(ctx context.Context, rec UserPushTokenRecord) error
	Revoke(ctx context.Context, userID types.UserID, deviceID string, platform PushPlatform, revokedAt time.Time) error
	ListActiveByUserID(ctx context.Context, userID types.UserID) ([]UserPushTokenRecord, error)
}

type NotificationPrefs struct {
	UserID              types.UserID
	EnablePush          bool
	EnableJourneyAlerts bool
	QuietHoursStart     string
	QuietHoursEnd       string
	UpdatedAt           time.Time
}

type UserNotificationPrefsStore interface {
	Get(ctx context.Context, userID types.UserID) (NotificationPrefs, bool, error)
	UpsertPrefs(ctx context.Context, prefs NotificationPrefs) error
}
