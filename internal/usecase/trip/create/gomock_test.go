package create_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/create"
)

func TestCreateWithMocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockrepository(ctrl)
	tx := NewMocktxManager(ctrl)
	metrics := NewMockmetrics(ctrl)
	var savedID uuid.UUID

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, value trip.Trip) error {
			savedID = value.ID
			return nil
		})
	repo.EXPECT().AddCreatedHistory(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) error {
			if id != savedID {
				t.Errorf("history ID = %s, want %s", id, savedID)
			}
			return nil
		})
	metrics.EXPECT().Created(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) {
			if id != savedID {
				t.Errorf("event ID = %s, want %s", id, savedID)
			}
		})

	got, err := create.New(repo, tx, metrics).Run(context.Background(), create.Input{
		UserID: uuid.New(), DriverID: uuid.New(), Price: 1450,
	})
	if err != nil || got.ID == uuid.Nil || got.ID != savedID {
		t.Errorf("Run() = %+v, %v; want saved trip", got, err)
	}
}
