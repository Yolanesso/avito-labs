package finish_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/httpserver/trip/finish"
	"github.com/Yolanesso/avito-labs/internal/model"
)

func TestFinishConflictWithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMocktripFinisher(ctrl)
	id := uuid.New()
	svc.EXPECT().Finish(gomock.Any(), id).Return(model.Trip{}, model.ErrTripCompleted)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
	recorder := httptest.NewRecorder()
	finish.New(svc).Handle(recorder, request, id)
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", recorder.Code)
	}
}
