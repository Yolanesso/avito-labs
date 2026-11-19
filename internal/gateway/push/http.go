package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("push service returned HTTP %d", e.StatusCode)
}

type Client struct {
	url    string
	client *http.Client
}

func New(baseURL string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("PUSH_HTTP_URL must be an HTTP(S) URL without credentials, query or fragment")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("PUSH_REQUEST_TIMEOUT must be positive")
	}
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: timeout, KeepAlive: timeout}).DialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       timeout,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
	}
	return &Client{
		url: strings.TrimRight(baseURL, "/") + "/api/v1/push",
		client: &http.Client{
			Transport: transport, Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (c *Client) RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error {
	ctx, span := otel.Tracer("tripgo/push").Start(ctx, "push.RequestPosition", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	body, err := json.Marshal(struct {
		RequestID   string            `json:"request_id"`
		RecipientID uuid.UUID         `json:"recipient_id"`
		Kind        string            `json:"kind"`
		Data        map[string]string `json:"data"`
	}{uuid.NewString(), driverID, "REQUEST_POSITION", map[string]string{"trip_id": tripID.String()}})
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(request.Header))
	response, err := c.client.Do(request)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "push transport")
		return fmt.Errorf("push transport: %w", err)
	}
	defer response.Body.Close()
	// Ограничиваем чтение; таймаут клиента действует и на чтение тела.
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusAccepted {
		err := &StatusError{StatusCode: response.StatusCode}
		span.RecordError(err)
		span.SetStatus(codes.Error, "push HTTP status")
		return err
	}
	if readErr != nil {
		return fmt.Errorf("read push response: %w", readErr)
	}
	return nil
}

func (c *Client) Close() {
	c.client.CloseIdleConnections()
}
