package repositories

import (
	"context"
	"errors"
	"time"

	userports "github.com/MathewM27/busTrack-alebus/application/user/ports"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
	"github.com/jackc/pgx/v5"
)

type PostgresUserPushNotificationsRepository struct {
	pool *db.Pool
}

func NewPostgresUserPushNotificationsRepository(pool *db.Pool) *PostgresUserPushNotificationsRepository {
	return &PostgresUserPushNotificationsRepository{pool: pool}
}

func (r *PostgresUserPushNotificationsRepository) UpsertToken(ctx context.Context, rec userports.UserPushTokenRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_push_tokens (user_id, device_id, platform, token, created_at, updated_at, last_seen_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL)
		ON CONFLICT (user_id, device_id, platform)
		DO UPDATE SET
			token=EXCLUDED.token,
			updated_at=EXCLUDED.updated_at,
			last_seen_at=EXCLUDED.last_seen_at,
			revoked_at=NULL
	`, string(rec.UserID), rec.DeviceID, string(rec.Platform), rec.Token, rec.CreatedAt, rec.UpdatedAt, rec.LastSeenAt)
	return err
}

func (r *PostgresUserPushNotificationsRepository) Revoke(ctx context.Context, userID types.UserID, deviceID string, platform userports.PushPlatform, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_push_tokens
		SET revoked_at=$4, updated_at=$4
		WHERE user_id=$1 AND device_id=$2 AND platform=$3 AND revoked_at IS NULL
	`, string(userID), deviceID, string(platform), revokedAt)
	return err
}

func (r *PostgresUserPushNotificationsRepository) ListActiveByUserID(ctx context.Context, userID types.UserID) ([]userports.UserPushTokenRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, device_id, platform, token, created_at, updated_at, last_seen_at, revoked_at
		FROM user_push_tokens
		WHERE user_id=$1 AND revoked_at IS NULL
		ORDER BY updated_at DESC
	`, string(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []userports.UserPushTokenRecord
	for rows.Next() {
		var rec userports.UserPushTokenRecord
		var platform string
		var lastSeen *time.Time
		var revoked *time.Time
		if err := rows.Scan(&rec.UserID, &rec.DeviceID, &platform, &rec.Token, &rec.CreatedAt, &rec.UpdatedAt, &lastSeen, &revoked); err != nil {
			return nil, err
		}
		rec.Platform = userports.PushPlatform(platform)
		rec.LastSeenAt = lastSeen
		rec.RevokedAt = revoked
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []userports.UserPushTokenRecord{}
	}
	return out, nil
}

func (r *PostgresUserPushNotificationsRepository) Get(ctx context.Context, userID types.UserID) (userports.NotificationPrefs, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT user_id, enable_push, enable_journey_alerts, quiet_hours_start, quiet_hours_end, updated_at
		FROM user_notification_prefs
		WHERE user_id=$1
	`, string(userID))

	var prefs userports.NotificationPrefs
	var qStart *string
	var qEnd *string
	if err := row.Scan(&prefs.UserID, &prefs.EnablePush, &prefs.EnableJourneyAlerts, &qStart, &qEnd, &prefs.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return userports.NotificationPrefs{}, false, nil
		}
		return userports.NotificationPrefs{}, false, err
	}
	if qStart != nil {
		prefs.QuietHoursStart = *qStart
	}
	if qEnd != nil {
		prefs.QuietHoursEnd = *qEnd
	}
	return prefs, true, nil
}

func (r *PostgresUserPushNotificationsRepository) UpsertPrefs(ctx context.Context, prefs userports.NotificationPrefs) error {
	var qStart any
	var qEnd any
	if prefs.QuietHoursStart != "" {
		qStart = prefs.QuietHoursStart
	}
	if prefs.QuietHoursEnd != "" {
		qEnd = prefs.QuietHoursEnd
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_notification_prefs (user_id, enable_push, enable_journey_alerts, quiet_hours_start, quiet_hours_end, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id)
		DO UPDATE SET
			enable_push=EXCLUDED.enable_push,
			enable_journey_alerts=EXCLUDED.enable_journey_alerts,
			quiet_hours_start=EXCLUDED.quiet_hours_start,
			quiet_hours_end=EXCLUDED.quiet_hours_end,
			updated_at=EXCLUDED.updated_at
	`, string(prefs.UserID), prefs.EnablePush, prefs.EnableJourneyAlerts, qStart, qEnd, prefs.UpdatedAt)
	return err
}

var _ userports.UserPushTokenStore = (*PostgresUserPushNotificationsRepository)(nil)
var _ userports.UserNotificationPrefsStore = (*PostgresUserPushNotificationsRepository)(nil)
