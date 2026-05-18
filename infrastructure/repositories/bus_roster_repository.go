package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/MathewM27/busTrack-alebus/application/journey/ports"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
	"github.com/jackc/pgx/v5"
)

// BusRosterRepository implements ports.BusRosterRepository using Postgres.
// Provides roster lookup capabilities for bus eligibility filtering.
type BusRosterRepository struct {
	pool *db.Pool
}

// NewBusRosterRepository creates a new Postgres-backed BusRosterRepository.
func NewBusRosterRepository(pool *db.Pool) *BusRosterRepository {
	return &BusRosterRepository{pool: pool}
}

// ListBusIDsByRoute returns bus IDs registered on a route.
// Supports optional filtering by operator and status.
func (r *BusRosterRepository) ListBusIDsByRoute(ctx context.Context, req ports.RosterQuery) ([]string, error) {
	if req.RouteID == "" {
		return nil, fmt.Errorf("routeId required")
	}

	// Build dynamic query with optional filters
	query := `SELECT bus_id FROM buses WHERE route_id = $1`
	args := []any{req.RouteID}
	argN := 2

	if req.OperatorID != "" {
		query += fmt.Sprintf(" AND operator_id = $%d", argN)
		args = append(args, req.OperatorID)
		argN++
	}
	if req.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, *req.Status)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query roster: %w", err)
	}
	defer rows.Close()

	var busIDs []string
	for rows.Next() {
		var busID string
		if err := rows.Scan(&busID); err != nil {
			return nil, fmt.Errorf("failed to scan bus_id: %w", err)
		}
		busIDs = append(busIDs, busID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return busIDs, nil
}

// Exists checks if a bus ID exists in the roster.
func (r *BusRosterRepository) Exists(ctx context.Context, busID string) (bool, error) {
	if busID == "" {
		return false, fmt.Errorf("busId required")
	}

	var exists string
	err := r.pool.QueryRow(ctx, `SELECT bus_id FROM buses WHERE bus_id = $1`, busID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check bus existence: %w", err)
	}

	return true, nil
}

// Compile-time interface check
var _ ports.BusRosterRepository = (*BusRosterRepository)(nil)
