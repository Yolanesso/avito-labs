package positions_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver/trip/positions"
)

func TestSaveValidation(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for _, body := range []string{
		`null`, `{}`, `{"coordinates":null,"recorded_at":"2026-09-01T12:00:00Z"}`,
		`{"coordinates":{},"recorded_at":"2026-09-01T12:00:00Z"}`,
		`{"coordinates":{"latitude":0},"recorded_at":"2026-09-01T12:00:00Z"}`,
		`{"coordinates":{"latitude":0,"longitude":0}}`,
		`{"coordinates":{"latitude":91,"longitude":0},"recorded_at":"2026-09-01T12:00:00Z"}`,
		`{"coordinates":{"latitude":0,"longitude":-181},"recorded_at":"2026-09-01T12:00:00Z"}`,
		`{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"not a date"}`,
		`{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-09-01T12:01:01Z"}`,
		`{"coordinates":{"latitude":0,"longitude":0},"recorded_at":null}`,
		`{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-09-01T12:00:00Z","extra":true}`,
		`{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-09-01T12:00:00Z"} {}`,
	} {
		t.Run(body, func(t *testing.T) {
			service := NewMockservice(gomock.NewController(t))
			h := positions.New(service, func() time.Time { return now })
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/positions", strings.NewReader(body))
			h.Save(w, r, uuid.New())
			if w.Code != http.StatusBadRequest || w.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("response = %d, %s", w.Code, w.Body.String())
			}
			var problem api.Problem
			if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil || problem.Code != "invalid_request" {
				t.Fatalf("problem = %v, %v", problem, err)
			}
		})
	}
}

func TestSaveResponses(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"created", nil, http.StatusCreated, ""},
		{"missing", trip.ErrTripNotFound, http.StatusNotFound, "trip_not_found"},
		{"completed", trip.ErrTripCompleted, http.StatusConflict, "trip_completed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := NewMockservice(gomock.NewController(t))
			id := uuid.New()
			p := trip.Position{TripID: id, Coordinates: trip.Coordinates{Latitude: -90, Longitude: 180}, RecordedAt: now.Add(time.Minute)}
			saved := p
			saved.ID = 42
			service.EXPECT().Save(gomock.Any(), p).Return(saved, tc.err)
			h := positions.New(service, func() time.Time { return now })
			r := httptest.NewRequest(http.MethodPost, "/positions", strings.NewReader(
				`{"coordinates":{"latitude":-90,"longitude":180},"recorded_at":"2026-09-01T12:01:00Z"}`,
			))
			w := httptest.NewRecorder()
			h.Save(w, r, id)
			if w.Code != tc.status {
				t.Fatalf("response = %d, %s", w.Code, w.Body.String())
			}
			if tc.err == nil {
				var got api.TripPosition
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Id != 42 || got.TripId != id || !got.RecordedAt.Equal(p.RecordedAt) {
					t.Fatalf("position = %v, %v", got, err)
				}
			} else if !strings.Contains(w.Body.String(), tc.code) {
				t.Fatalf("missing code: %s", w.Body.String())
			}
		})
	}
}

func TestListEmptyAndNotFound(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(map[bool]string{true: "empty", false: "missing"}[exists], func(t *testing.T) {
			service := NewMockservice(gomock.NewController(t))
			id := uuid.New()
			var err error
			if !exists {
				err = trip.ErrTripNotFound
			}
			service.EXPECT().List(gomock.Any(), id).Return(nil, err)
			w := httptest.NewRecorder()
			positions.New(service, time.Now).List(w, httptest.NewRequest(http.MethodGet, "/positions", nil), id)
			if exists {
				if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"positions":[]}` {
					t.Fatalf("response = %d, %s", w.Code, w.Body.String())
				}
			} else if w.Code != http.StatusNotFound {
				t.Fatalf("response = %d", w.Code)
			}
		})
	}
}
