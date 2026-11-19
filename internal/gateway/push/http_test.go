//go:generate go tool mockgen -source=http_test.go -destination=mocks_test.go -package=push

package push

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

type transport interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

type responseBody interface {
	Read([]byte) (int, error)
	Close() error
}

var (
	_ transport    = (*Mocktransport)(nil)
	_ responseBody = (*MockresponseBody)(nil)
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestRequestContractAndTracePropagation(t *testing.T) {
	driverID, tripID := uuid.New(), uuid.New()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled,
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/push" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request = %s %s, content type = %s", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		var body struct {
			RequestID   string            `json:"request_id"`
			RecipientID uuid.UUID         `json:"recipient_id"`
			Kind        string            `json:"kind"`
			Data        map[string]string `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.RequestID == "" || body.RecipientID != driverID || body.Kind != "REQUEST_POSITION" || body.Data["trip_id"] != tripID.String() {
			t.Errorf("body = %+v", body)
		}
		if r.Header.Get("Traceparent") == "" {
			t.Error("missing W3C trace context")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client, err := New(server.URL+"/", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	if err := client.RequestPosition(ctx, driverID, tripID); err != nil {
		t.Fatal(err)
	}
}

func TestStatusErrorsAndNoRetry(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(code)
			}))
			client, err := New(server.URL, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			err = client.RequestPosition(context.Background(), uuid.New(), uuid.New())
			client.Close()
			server.Close()
			var status *StatusError
			if !errors.As(err, &status) || status.StatusCode != code || calls != 1 {
				t.Fatalf("error = %v, calls = %d", err, calls)
			}
		})
	}
}

func TestTimeoutAndTransportError(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
	}))
	defer server.Close()
	defer close(release)
	client, err := New(server.URL, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result := make(chan error, 1)
	go func() {
		result <- client.RequestPosition(context.Background(), uuid.New(), uuid.New())
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach server")
	}
	select {
	case err := <-result:
		var status *StatusError
		if err == nil || errors.As(err, &status) {
			t.Fatalf("expected transport timeout, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client timeout did not fire")
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, url := range []string{"", "localhost:8090", "ftp://example.org", "http://user:pass@example.org", "http://example.org?token=x"} {
		if _, err := New(url, time.Second); err == nil {
			t.Error("accepted invalid URL")
		}
	}
	if _, err := New("http://example.org", 0); err == nil {
		t.Error("accepted zero timeout")
	}
}

func TestResponseBodyClosedOnSuccessStatusAndReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		readErr   error
		wantError bool
	}{
		{"accepted", http.StatusAccepted, io.EOF, false},
		{"client error", http.StatusBadRequest, io.EOF, true},
		{"server error", http.StatusServiceUnavailable, io.EOF, true},
		{"read error", http.StatusAccepted, errors.New("broken body"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			body, roundTripper := NewMockresponseBody(ctrl), NewMocktransport(ctrl)
			body.EXPECT().Read(gomock.Any()).Return(0, tc.readErr)
			body.EXPECT().Close().Return(nil)
			roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{StatusCode: tc.status, Body: body}, nil)
			client, err := New("http://push.example.org", time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.client.Transport = roundTripper
			err = client.RequestPosition(context.Background(), uuid.New(), uuid.New())
			if (err != nil) != tc.wantError {
				t.Fatalf("request error = %v", err)
			}
		})
	}
}
