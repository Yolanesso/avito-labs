package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/get"
)

func TestGetWithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockrepository(ctrl)
	id := uuid.New()
	repo.EXPECT().GetByID(gomock.Any(), id).Return(trip.Trip{}, trip.ErrTripNotFound)

	_, err := get.New(repo).Run(context.Background(), id)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Errorf("Run() error = %v, want ErrTripNotFound", err)
	}
}
