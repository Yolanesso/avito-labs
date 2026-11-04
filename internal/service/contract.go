//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=service_test

package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/model"
)

type tripRepository interface {
	Create(ctx context.Context, trip model.Trip) error
	AddCreatedHistory(ctx context.Context, tripID uuid.UUID) error
	GetByID(ctx context.Context, tripID uuid.UUID) (model.Trip, error)
	Finish(ctx context.Context, tripID uuid.UUID, finishedAt time.Time) error
	AddCompletedHistory(ctx context.Context, tripID uuid.UUID) error
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
