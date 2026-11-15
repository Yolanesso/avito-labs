package finish_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/finish"
)

func TestFinishCompletedWithMocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockrepository(ctrl)
	tx := NewMocktxManager(ctrl)
	metrics := NewMockmetrics(ctrl)
	id := uuid.New()

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
	repo.EXPECT().GetByID(gomock.Any(), id).
		Return(trip.Trip{ID: id, Status: trip.StatusCompleted}, nil)
	// Finish, история и событие не ожидаются: gomock остановит тест, если они вызовутся.
	_, err := finish.New(repo, tx, metrics).Run(context.Background(), id)
	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Errorf("Run() error = %v, want ErrTripCompleted", err)
	}
}
