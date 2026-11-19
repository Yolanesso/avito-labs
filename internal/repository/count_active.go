package repository

import (
	"context"
	"fmt"

	"github.com/Yolanesso/avito-labs/internal/txmanager"
)

func (r *TripRepository) CountActive(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var count int64
	err := txmanager.ExecutorFromContext(ctx, r.pool).
		QueryRow(ctx, "SELECT count(*) FROM trips WHERE status = 'active'").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}
