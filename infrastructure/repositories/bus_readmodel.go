package repositories

import (
	"context"
	"fmt"
	"time"

	busreadmodel "github.com/MathewM27/busTrack-alebus/application/bus/readmodel"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
)

// PostgresBusReadModel implements busreadmodel.BusReader using Postgres.
// This is a read-optimized query adapter for bus position lookups.
//
// Note: For production, this should be backed by Redis for sub-millisecond reads.
// Current implementation queries Postgres directly.
type PostgresBusReadModel struct {
	pool *db.Pool
}

func safeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// NewPostgresBusReadModel creates a new Postgres-backed BusReader.
func NewPostgresBusReadModel(pool *db.Pool) *PostgresBusReadModel {
	return &PostgresBusReadModel{pool: pool}
}

// GetBus retrieves a single bus by ID.
func (r *PostgresBusReadModel) GetBus(ctx context.Context, req busreadmodel.GetBusRequest) (busreadmodel.BusDTO, bool, error) {
	ctx, cancel := context.WithTimeout(safeContext(ctx), 5*time.Second)
	defer cancel()

	var dto busreadmodel.BusDTO
	var posTimestamp time.Time
	var updatedAt time.Time
	var terminalArrivalTime *time.Time

	err := r.pool.QueryRow(ctx, `
		SELECT bus_id, operator_id, route_id, 
		       position_lat, position_lon, position_timestamp, position_accuracy, position_speed_kmh,
		       stop_index, direction, status,
		       is_at_terminal, terminal_arrival_time,
		       updated_at
		FROM buses
		WHERE bus_id = $1
	`, req.BusID).Scan(
		&dto.BusID, &dto.OperatorID, &dto.RouteID,
		&dto.Position.Lat, &dto.Position.Lon, &posTimestamp, &dto.Position.Accuracy, &dto.Position.SpeedKmh,
		&dto.StopIndex, &dto.Direction, &dto.Status,
		&dto.IsAtTerminal, &terminalArrivalTime,
		&updatedAt,
	)
	if err != nil {
		return busreadmodel.BusDTO{}, false, nil // Not found
	}

	dto.Position.Timestamp = posTimestamp.UnixMilli()
	dto.UpdatedAt = updatedAt.Format(time.RFC3339)
	if terminalArrivalTime != nil {
		dto.TerminalArrivalTime = terminalArrivalTime.Format(time.RFC3339)
	}

	return dto, true, nil
}

// ListBuses retrieves buses matching the filter criteria.
func (r *PostgresBusReadModel) ListBuses(ctx context.Context, req busreadmodel.ListBusesRequest) ([]busreadmodel.BusDTO, error) {
	ctx, cancel := context.WithTimeout(safeContext(ctx), 5*time.Second)
	defer cancel()

	// Build query with optional filters
	query := `
		SELECT bus_id, operator_id, route_id, 
		       position_lat, position_lon, position_timestamp, position_accuracy, position_speed_kmh,
		       stop_index, direction, status,
		       is_at_terminal, terminal_arrival_time,
		       updated_at
		FROM buses
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIdx := 1

	if req.RouteID != "" {
		query += fmt.Sprintf(" AND route_id = $%d", argIdx)
		args = append(args, req.RouteID)
		argIdx++
	}

	if req.OperatorID != "" {
		query += fmt.Sprintf(" AND operator_id = $%d", argIdx)
		args = append(args, req.OperatorID)
		argIdx++
	}

	if req.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *req.Status)
	}

	query += " ORDER BY bus_id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query buses: %w", err)
	}
	defer rows.Close()

	var buses []busreadmodel.BusDTO
	for rows.Next() {
		var dto busreadmodel.BusDTO
		var posTimestamp time.Time
		var updatedAt time.Time
		var terminalArrivalTime *time.Time

		if err := rows.Scan(
			&dto.BusID, &dto.OperatorID, &dto.RouteID,
			&dto.Position.Lat, &dto.Position.Lon, &posTimestamp, &dto.Position.Accuracy, &dto.Position.SpeedKmh,
			&dto.StopIndex, &dto.Direction, &dto.Status,
			&dto.IsAtTerminal, &terminalArrivalTime,
			&updatedAt,
		); err != nil {
			continue
		}

		dto.Position.Timestamp = posTimestamp.UnixMilli()
		dto.UpdatedAt = updatedAt.Format(time.RFC3339)
		if terminalArrivalTime != nil {
			dto.TerminalArrivalTime = terminalArrivalTime.Format(time.RFC3339)
		}

		buses = append(buses, dto)
	}

	if buses == nil {
		buses = []busreadmodel.BusDTO{}
	}

	return buses, nil
}

// Compile-time interface check
var _ busreadmodel.BusReader = (*PostgresBusReadModel)(nil)
