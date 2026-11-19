package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/txmanager"
)

// TouchPosition блокирует строку до конца транзакции, в том числе против Finish.
// Сравнение в SQL не даёт запоздавшей точке уменьшить last_position_at.
func (r *TripRepository) TouchPosition(ctx context.Context, tripID uuid.UUID, at time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	tag, err := executor.Exec(ctx, `
		UPDATE trips
		SET last_position_at = GREATEST(last_position_at, $2), updated_at = now()
		WHERE id = $1 AND status = 'active'`, tripID, at)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}
	if tag.RowsAffected() != 0 {
		return nil
	}
	var status string
	err = executor.QueryRow(ctx, "SELECT status FROM trips WHERE id = $1", tripID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return trip.ErrTripNotFound
	}
	if err != nil {
		return fmt.Errorf("check position trip: %w", err)
	}
	return trip.ErrTripCompleted
}

func (r *TripRepository) InsertPosition(ctx context.Context, position trip.Position) (trip.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	err := executor.QueryRow(ctx, `
		INSERT INTO trip_positions (trip_id, latitude, longitude, recorded_at)
		VALUES ($1, $2, $3, $4) RETURNING id, recorded_at`,
		position.TripID, position.Coordinates.Latitude,
		position.Coordinates.Longitude, position.RecordedAt,
	).Scan(&position.ID, &position.RecordedAt)
	if err != nil {
		return trip.Position{}, fmt.Errorf("insert position: %w", err)
	}
	return position, nil
}

func (r *TripRepository) ListPositions(ctx context.Context, tripID uuid.UUID) ([]trip.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	rows, err := executor.Query(ctx, `
		SELECT id, trip_id, latitude, longitude, recorded_at
		FROM trip_positions WHERE trip_id = $1 ORDER BY recorded_at, id`, tripID)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	defer rows.Close()
	positions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (trip.Position, error) {
		var p trip.Position
		err := row.Scan(&p.ID, &p.TripID, &p.Coordinates.Latitude, &p.Coordinates.Longitude, &p.RecordedAt)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("read positions: %w", err)
	}
	return positions, nil
}

func (r *TripRepository) FindStaleTrips(ctx context.Context, staleAfter time.Duration, limit int, after *trip.StaleTripCursor) ([]trip.StaleTrip, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("stale trips batch size must be positive")
	}
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	query := `
		SELECT id, driver_id, COALESCE(last_position_at, started_at) FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at) < now() - ($1::double precision * interval '1 second')`
	args := []any{staleAfter.Seconds(), limit}
	if after != nil {
		// Отдельная форма запроса без OR ... IS NULL сохраняет range scan
		// по курсору и для generic plan подготовленного выражения.
		query += ` AND (COALESCE(last_position_at, started_at), id) > ($3::timestamptz, $4::uuid)`
		args = append(args, after.PositionAt, after.ID)
	}
	query += ` ORDER BY COALESCE(last_position_at, started_at), id LIMIT $2`
	rows, err := executor.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find stale trips: %w", err)
	}
	defer rows.Close()
	trips, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (trip.StaleTrip, error) {
		var t trip.StaleTrip
		err := row.Scan(&t.ID, &t.DriverID, &t.PositionAt)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("read stale trips: %w", err)
	}
	return trips, nil
}
