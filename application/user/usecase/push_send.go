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
	ErrInvalidPushMessage = errors.New("invalid push message")
)

type SendTestPushNotificationRequest struct {
	UserID types.UserID
	Title  string
	Body   string
}

// SendTestPushNotificationUseCase sends a simple push notification to all active
// tokens for the given user.
//
// This is intended for development/ops testing and future worker reuse.
type SendTestPushNotificationUseCase struct {
	tokens userports.UserPushTokenStore
	prefs  userports.UserNotificationPrefsStore
	sender userports.PushSender
	now    func() time.Time
}

func NewSendTestPushNotificationUseCase(
	tokens userports.UserPushTokenStore,
	prefs userports.UserNotificationPrefsStore,
	sender userports.PushSender,
) *SendTestPushNotificationUseCase {
	return &SendTestPushNotificationUseCase{tokens: tokens, prefs: prefs, sender: sender, now: func() time.Time { return time.Now().UTC() }}
}

func (uc *SendTestPushNotificationUseCase) Send(ctx context.Context, req SendTestPushNotificationRequest) (int, error) {
	if uc == nil || uc.tokens == nil || uc.prefs == nil || uc.sender == nil {
		return 0, ErrStoreUnavailable
	}
	if strings.TrimSpace(string(req.UserID)) == "" {
		return 0, ErrInvalidUserID
	}
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)
	if title == "" || body == "" {
		return 0, ErrInvalidPushMessage
	}

	p, found, err := uc.prefs.Get(ctx, req.UserID)
	if err != nil {
		return 0, err
	}
	if found && !p.EnablePush {
		return 0, nil
	}

	toks, err := uc.tokens.ListActiveByUserID(ctx, req.UserID)
	if err != nil {
		return 0, err
	}
	if len(toks) == 0 {
		return 0, nil
	}

	msgs := make([]userports.PushMessage, 0, len(toks))
	byToken := make(map[string]userports.UserPushTokenRecord, len(toks))
	for _, t := range toks {
		if strings.TrimSpace(t.Token) == "" {
			continue
		}
		byToken[t.Token] = t
		msgs = append(msgs, userports.PushMessage{
			To:    t.Token,
			Title: title,
			Body:  body,
			Data: map[string]any{
				"type":   "test",
				"userId": string(req.UserID),
			},
		})
	}
	if len(msgs) == 0 {
		return 0, nil
	}

	if s, ok := uc.sender.(userports.PushSenderWithReport); ok {
		rep, err := s.SendWithReport(ctx, msgs)
		if err != nil {
			return 0, err
		}
		uc.revokeInvalidTokensBestEffort(ctx, byToken, rep.InvalidTokens)
		return len(msgs), nil
	}

	if err := uc.sender.Send(ctx, msgs); err != nil {
		return 0, err
	}
	return len(msgs), nil
}

func (uc *SendTestPushNotificationUseCase) revokeInvalidTokensBestEffort(ctx context.Context, byToken map[string]userports.UserPushTokenRecord, invalidTokens []string) {
	if uc == nil || uc.tokens == nil {
		return
	}
	if len(invalidTokens) == 0 {
		return
	}
	now := uc.now()
	for _, tok := range invalidTokens {
		rec, ok := byToken[tok]
		if !ok {
			continue
		}
		platform := rec.Platform
		if platform == "" {
			platform = userports.PushPlatformExpo
		}
		_ = uc.tokens.Revoke(ctx, rec.UserID, rec.DeviceID, platform, now)
	}
}
