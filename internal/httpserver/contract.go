//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=httpserver_test

package httpserver

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/service"
)

type tripService interface {
	Create(ctx context.Context, input service.CreateTripInput) (model.Trip, error)
	GetByID(ctx context.Context, tripID uuid.UUID) (model.Trip, error)
	Finish(ctx context.Context, tripID uuid.UUID) (model.Trip, error)
}
