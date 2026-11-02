package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

func TestFinish(t *testing.T) {
	finishedAt := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		status  trip.Status
		wantErr error
	}{
		{
			name:   "active trip can be finished",
			status: trip.StatusActive,
		},
		{
			name:    "completed trip cannot be finished again",
			status:  trip.StatusCompleted,
			wantErr: trip.ErrTripCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trip.Trip{Status: tt.status}

			err := got.Finish(finishedAt)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Finish() error = %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				if got.Status != tt.status || got.FinishedAt != nil {
					t.Fatal("failed Finish() changed the trip")
				}
				return
			}

			if got.Status != trip.StatusCompleted {
				t.Errorf("status = %q, want completed", got.Status)
			}
			if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
				t.Errorf("finished_at = %v, want %v", got.FinishedAt, finishedAt)
			}
			if !got.UpdatedAt.Equal(finishedAt) {
				t.Errorf("updated_at = %v, want %v", got.UpdatedAt, finishedAt)
			}
		})
	}
}
