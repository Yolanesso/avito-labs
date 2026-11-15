package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/get"
)

type repositoryStub struct {
	value trip.Trip
	err   error
}

func (r repositoryStub) GetByID(_ context.Context, _ uuid.UUID) (trip.Trip, error) {
	return r.value, r.err
}

func TestGet(t *testing.T) {
	id := uuid.New()
	uc := get.New(repositoryStub{value: trip.Trip{ID: id}})
	got, err := uc.Run(context.Background(), id)
	if err != nil || got.ID != id {
		t.Errorf("Run() = %+v, %v; want ID %s", got, err, id)
	}
}

func TestGetNotFound(t *testing.T) {
	uc := get.New(repositoryStub{err: trip.ErrTripNotFound})
	_, err := uc.Run(context.Background(), uuid.New())
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Errorf("Run() error = %v, want ErrTripNotFound", err)
	}
}
