package positions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

func TestSaveCommitsBeforeCancelAndMetric(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo, tx := NewMockrepository(ctrl), NewMocktxManager(ctrl)
	queue, metrics := NewMockcanceller(ctrl), NewMockmetrics(ctrl)
	p := trip.Position{TripID: uuid.New(), RecordedAt: time.Now()}
	saved := p
	saved.ID = 42
	committed := false
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		if err := fn(ctx); err != nil {
			return err
		}
		committed = true
		return nil
	})
	gomock.InOrder(
		repo.EXPECT().TouchPosition(gomock.Any(), p.TripID, p.RecordedAt).Return(nil),
		repo.EXPECT().InsertPosition(gomock.Any(), p).Return(saved, nil),
		queue.EXPECT().Cancel(gomock.Any(), p.TripID).Do(func(context.Context, uuid.UUID) {
			if !committed {
				t.Error("cancel called before commit")
			}
		}),
		metrics.EXPECT().Saved(gomock.Any()),
	)
	got, err := New(repo, tx, queue, metrics).Save(context.Background(), p)
	if err != nil || got != saved {
		t.Fatalf("save = %v, %v", got, err)
	}
}

func TestSaveFailureDoesNotCancelOrCount(t *testing.T) {
	const updateStage = "update"
	for _, stage := range []string{updateStage, "insert", "commit"} {
		t.Run(stage, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo, tx := NewMockrepository(ctrl), NewMocktxManager(ctrl)
			queue, metrics := NewMockcanceller(ctrl), NewMockmetrics(ctrl)
			p := trip.Position{TripID: uuid.New(), RecordedAt: time.Now()}
			failure := errors.New("storage failed")
			tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				if err := fn(ctx); err != nil {
					return err
				}
				return failure
			})
			updateErr := error(nil)
			if stage == updateStage {
				updateErr = failure
			}
			repo.EXPECT().TouchPosition(gomock.Any(), p.TripID, p.RecordedAt).Return(updateErr)
			if stage != updateStage {
				insertErr := error(nil)
				if stage == "insert" {
					insertErr = failure
				}
				repo.EXPECT().InsertPosition(gomock.Any(), p).Return(p, insertErr)
			}
			_, err := New(repo, tx, queue, metrics).Save(context.Background(), p)
			if !errors.Is(err, failure) {
				t.Fatalf("save error = %v", err)
			}
		})
	}
}

func TestListRequiresExistingTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockrepository(ctrl)
	id := uuid.New()
	repo.EXPECT().GetByID(gomock.Any(), id).Return(trip.Trip{}, trip.ErrTripNotFound)
	_, err := New(repo, nil, nil, nil).List(context.Background(), id)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatal(err)
	}
}
