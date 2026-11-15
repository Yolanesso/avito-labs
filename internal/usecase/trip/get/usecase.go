package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type UseCase struct {
	repository repository
}

func New(repository repository) *UseCase {
	return &UseCase{repository: repository}
}

func (u *UseCase) Run(ctx context.Context, tripID uuid.UUID) (trip.Trip, error) {
	return u.repository.GetByID(ctx, tripID)
}
