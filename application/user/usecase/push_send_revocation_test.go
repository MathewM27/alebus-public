package userusecase

import (
	"context"
	"testing"
	"time"

	userports "github.com/MathewM27/busTrack-alebus/application/user/ports"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type fakeUserPrefsStore struct {
	prefs userports.NotificationPrefs
	found bool
	err   error
}

func (s *fakeUserPrefsStore) Get(ctx context.Context, userID types.UserID) (userports.NotificationPrefs, bool, error) {
	return s.prefs, s.found, s.err
}

func (s *fakeUserPrefsStore) UpsertPrefs(ctx context.Context, prefs userports.NotificationPrefs) error {
	panic("not needed")
}

type fakeUserTokenStore struct {
	tokens    []userports.UserPushTokenRecord
	revoked   []userports.UserPushTokenRecord
	listErr   error
	revokeErr error
}

func (s *fakeUserTokenStore) UpsertToken(ctx context.Context, rec userports.UserPushTokenRecord) error {
	panic("not needed")
}

func (s *fakeUserTokenStore) Revoke(ctx context.Context, userID types.UserID, deviceID string, platform userports.PushPlatform, revokedAt time.Time) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.revoked = append(s.revoked, userports.UserPushTokenRecord{UserID: userID, DeviceID: deviceID, Platform: platform})
	return nil
}

func (s *fakeUserTokenStore) ListActiveByUserID(ctx context.Context, userID types.UserID) ([]userports.UserPushTokenRecord, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.tokens, nil
}

type fakeSenderWithReportForUser struct {
	report userports.PushSendReport
}

func (s *fakeSenderWithReportForUser) Send(ctx context.Context, messages []userports.PushMessage) error {
	return nil
}

func (s *fakeSenderWithReportForUser) SendWithReport(ctx context.Context, messages []userports.PushMessage) (userports.PushSendReport, error) {
	return s.report, nil
}

func TestSendTestPushNotification_RevokesInvalidTokens(t *testing.T) {
	store := &fakeUserTokenStore{tokens: []userports.UserPushTokenRecord{{
		UserID:   types.UserID("u1"),
		DeviceID: "d1",
		Platform: userports.PushPlatformExpo,
		Token:    "ExponentPushToken[abc]",
	}}}
	prefs := &fakeUserPrefsStore{found: false}
	sender := &fakeSenderWithReportForUser{report: userports.PushSendReport{InvalidTokens: []string{"ExponentPushToken[abc]"}}}

	uc := NewSendTestPushNotificationUseCase(store, prefs, sender)
	uc.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

	count, err := uc.Send(context.Background(), SendTestPushNotificationRequest{UserID: types.UserID("u1"), Title: "t", Body: "b"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}
	if got := len(store.revoked); got != 1 {
		t.Fatalf("expected 1 revoked token, got %d", got)
	}
}
