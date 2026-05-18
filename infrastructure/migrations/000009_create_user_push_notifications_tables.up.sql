-- User push notification foundations.
-- Stores per-device push tokens (Expo) and per-user notification preferences.

CREATE TABLE user_push_tokens (
	user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	device_id TEXT NOT NULL,
	platform TEXT NOT NULL,
	token TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_seen_at TIMESTAMPTZ,
	revoked_at TIMESTAMPTZ,
	PRIMARY KEY (user_id, device_id, platform)
);

CREATE INDEX idx_user_push_tokens_user_id ON user_push_tokens (user_id);
CREATE INDEX idx_user_push_tokens_token ON user_push_tokens (token);

COMMENT ON TABLE user_push_tokens IS 'Push notification device tokens per user/device/platform (e.g., Expo push tokens)';


CREATE TABLE user_notification_prefs (
	user_id TEXT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
	enable_push BOOLEAN NOT NULL DEFAULT TRUE,
	enable_journey_alerts BOOLEAN NOT NULL DEFAULT TRUE,
	quiet_hours_start TEXT,
	quiet_hours_end TEXT,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_notification_prefs IS 'Per-user notification preferences used by background workers';
