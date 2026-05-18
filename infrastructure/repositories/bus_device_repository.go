package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/MathewM27/busTrack-alebus/application/bus/ports"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
	"github.com/jackc/pgx/v5"
)

// BusDeviceRepository is a Postgres-backed store for bus device provisioning.
// It implements the application-layer ports.BusDeviceStore.
type BusDeviceRepository struct {
	pool *db.Pool
}

func NewBusDeviceRepository(pool *db.Pool) *BusDeviceRepository {
	return &BusDeviceRepository{pool: pool}
}

func (r *BusDeviceRepository) GetActiveByBusID(ctx context.Context, busID types.BusID) (ports.BusDeviceRecord, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT device_id, bus_id, mqtt_username, mqtt_password_hash, created_at, revoked_at, last_seen_at
		FROM bus_devices
		WHERE bus_id=$1 AND revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, string(busID))

	var rec ports.BusDeviceRecord
	var revokedAt *time.Time
	var lastSeen *time.Time
	if err := row.Scan(&rec.DeviceID, &rec.BusID, &rec.MQTTUsername, &rec.MQTTPasswordHash, &rec.CreatedAt, &revokedAt, &lastSeen); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.BusDeviceRecord{}, false, nil
		}
		return ports.BusDeviceRecord{}, false, err
	}
	rec.RevokedAt = revokedAt
	rec.LastSeenAt = lastSeen
	return rec, true, nil
}

func (r *BusDeviceRepository) GetActiveByUsername(ctx context.Context, username string) (ports.BusDeviceRecord, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT device_id, bus_id, mqtt_username, mqtt_password_hash, created_at, revoked_at, last_seen_at
		FROM bus_devices
		WHERE mqtt_username=$1 AND revoked_at IS NULL
		LIMIT 1
	`, username)

	var rec ports.BusDeviceRecord
	var revokedAt *time.Time
	var lastSeen *time.Time
	if err := row.Scan(&rec.DeviceID, &rec.BusID, &rec.MQTTUsername, &rec.MQTTPasswordHash, &rec.CreatedAt, &revokedAt, &lastSeen); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.BusDeviceRecord{}, false, nil
		}
		return ports.BusDeviceRecord{}, false, err
	}
	rec.RevokedAt = revokedAt
	rec.LastSeenAt = lastSeen
	return rec, true, nil
}

func (r *BusDeviceRepository) Create(ctx context.Context, rec ports.BusDeviceRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bus_devices (device_id, bus_id, mqtt_username, mqtt_password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, rec.DeviceID, string(rec.BusID), rec.MQTTUsername, rec.MQTTPasswordHash, rec.CreatedAt)
	return err
}

func (r *BusDeviceRepository) Revoke(ctx context.Context, deviceID string, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE bus_devices
		SET revoked_at=$2
		WHERE device_id=$1 AND revoked_at IS NULL
	`, deviceID, revokedAt)
	return err
}

func (r *BusDeviceRepository) UpdateLastSeen(ctx context.Context, deviceID string, seenAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE bus_devices
		SET last_seen_at=$2
		WHERE device_id=$1
	`, deviceID, seenAt)
	return err
}

var _ ports.BusDeviceStore = (*BusDeviceRepository)(nil)
