package userusecase

import (
	"context"
	"errors"
	"strings"
	"time"

	userports "github.com/MathewM27/busTrack-alebus/application/user/ports"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

var (
	ErrInvalidUserID    = errors.New("invalid userId")
	ErrInvalidDeviceID  = errors.New("invalid deviceId")
	ErrInvalidPushToken = errors.New("invalid push token")
	ErrInvalidPlatform  = errors.New("invalid platform")
	ErrStoreUnavailable = errors.New("store unavailable")
)

type RegisterPushTokenRequest struct {
	UserID   types.UserID
	DeviceID string
	Platform userports.PushPlatform
	Token    string
}

type RegisterPushTokenUseCase struct {
	store userports.UserPushTokenStore
	now   func() time.Time
}

func NewRegisterPushTokenUseCase(store userports.UserPushTokenStore) *RegisterPushTokenUseCase {
	return &RegisterPushTokenUseCase{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (uc *RegisterPushTokenUseCase) Register(ctx context.Context, req RegisterPushTokenRequest) error {
	if uc == nil || uc.store == nil {
		return ErrStoreUnavailable
	}
	if strings.TrimSpace(string(req.UserID)) == "" {
		return ErrInvalidUserID
	}
	if strings.TrimSpace(req.DeviceID) == "" {
		return ErrInvalidDeviceID
	}
	if strings.TrimSpace(req.Token) == "" {
		return ErrInvalidPushToken
	}
	if req.Platform == "" {
		req.Platform = userports.PushPlatformExpo
	}
	if req.Platform != userports.PushPlatformExpo {
		return ErrInvalidPlatform
	}

	now := uc.now()
	rec := userports.UserPushTokenRecord{
		UserID:     req.UserID,
		DeviceID:   req.DeviceID,
		Platform:   req.Platform,
		Token:      req.Token,
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSeenAt: ptrTime(now),
	}
	return uc.store.UpsertToken(ctx, rec)
}

type RevokePushTokenRequest struct {
	UserID   types.UserID
	DeviceID string
	Platform userports.PushPlatform
}

type RevokePushTokenUseCase struct {
	store userports.UserPushTokenStore
	now   func() time.Time
}

func NewRevokePushTokenUseCase(store userports.UserPushTokenStore) *RevokePushTokenUseCase {
	return &RevokePushTokenUseCase{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (uc *RevokePushTokenUseCase) Revoke(ctx context.Context, req RevokePushTokenRequest) error {
	if uc == nil || uc.store == nil {
		return ErrStoreUnavailable
	}
	if strings.TrimSpace(string(req.UserID)) == "" {
		return ErrInvalidUserID
	}
	if strings.TrimSpace(req.DeviceID) == "" {
		return ErrInvalidDeviceID
	}
	if req.Platform == "" {
		req.Platform = userports.PushPlatformExpo
	}
	if req.Platform != userports.PushPlatformExpo {
		return ErrInvalidPlatform
	}

	return uc.store.Revoke(ctx, req.UserID, req.DeviceID, req.Platform, uc.now())
}

type GetNotificationPrefsUseCase struct {
	store userports.UserNotificationPrefsStore
}

func NewGetNotificationPrefsUseCase(store userports.UserNotificationPrefsStore) *GetNotificationPrefsUseCase {
	return &GetNotificationPrefsUseCase{store: store}
}

func (uc *GetNotificationPrefsUseCase) Get(ctx context.Context, userID types.UserID) (userports.NotificationPrefs, bool, error) {
	if uc == nil || uc.store == nil {
		return userports.NotificationPrefs{}, false, ErrStoreUnavailable
	}
	if strings.TrimSpace(string(userID)) == "" {
		return userports.NotificationPrefs{}, false, ErrInvalidUserID
	}
	return uc.store.Get(ctx, userID)
}

type UpsertNotificationPrefsRequest struct {
	UserID              types.UserID
	EnablePush          bool
	EnableJourneyAlerts bool
	QuietHoursStart     string
	QuietHoursEnd       string
}

type UpsertNotificationPrefsUseCase struct {
	store userports.UserNotificationPrefsStore
	now   func() time.Time
}

func NewUpsertNotificationPrefsUseCase(store userports.UserNotificationPrefsStore) *UpsertNotificationPrefsUseCase {
	return &UpsertNotificationPrefsUseCase{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (uc *UpsertNotificationPrefsUseCase) Upsert(ctx context.Context, req UpsertNotificationPrefsRequest) error {
	if uc == nil || uc.store == nil {
		return ErrStoreUnavailable
	}
	if strings.TrimSpace(string(req.UserID)) == "" {
		return ErrInvalidUserID
	}

	prefs := userports.NotificationPrefs{
		UserID:              req.UserID,
		EnablePush:          req.EnablePush,
		EnableJourneyAlerts: req.EnableJourneyAlerts,
		QuietHoursStart:     strings.TrimSpace(req.QuietHoursStart),
		QuietHoursEnd:       strings.TrimSpace(req.QuietHoursEnd),
		UpdatedAt:           uc.now(),
	}
	return uc.store.UpsertPrefs(ctx, prefs)
}

func ptrTime(t time.Time) *time.Time { return &t }
