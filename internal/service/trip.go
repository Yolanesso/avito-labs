package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/create"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/finish"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/get"
)

type CreateTripInput = create.Input

type tripMetrics interface {
	Created(context.Context, uuid.UUID)
	Completed(context.Context, uuid.UUID)
}

type TripService struct {
	repository tripRepository
	txManager  txManager
	metrics    tripMetrics
}

func NewTripService(
	repository tripRepository,
	txManager txManager,
) *TripService {
	return &TripService{
		repository: repository,
		txManager:  txManager,
	}
}

func (s *TripService) SetMetrics(metrics tripMetrics) {
	s.metrics = metrics
}

func (s *TripService) Create(
	ctx context.Context,
	input CreateTripInput,
) (model.Trip, error) {
	return create.New(s.repository, s.txManager, s.metrics).Run(ctx, input)
}

func (s *TripService) GetByID(
	ctx context.Context,
	tripID uuid.UUID,
) (model.Trip, error) {
	return get.New(s.repository).Run(ctx, tripID)
}

func (s *TripService) Finish(
	ctx context.Context,
	tripID uuid.UUID,
) (model.Trip, error) {
	return finish.New(s.repository, s.txManager, s.metrics).Run(ctx, tripID)
}
