package httpserver_test

import (
	"log/slog"
	"testing"
	"time"

	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver"
)

func TestServerUsesConfiguredTimeouts(t *testing.T) {
	timeouts := httpserver.Timeouts{
		ReadTimeout: 2 * time.Second, ReadHeaderTimeout: time.Second,
		WriteTimeout: 3 * time.Second, IdleTimeout: 4 * time.Second,
	}
	server := httpserver.New(":0", &api.Unimplemented{}, slog.Default(), timeouts)
	if server.ReadTimeout != timeouts.ReadTimeout || server.ReadHeaderTimeout != timeouts.ReadHeaderTimeout ||
		server.WriteTimeout != timeouts.WriteTimeout || server.IdleTimeout != timeouts.IdleTimeout {
		t.Fatal("HTTP server did not receive configured timeouts")
	}
}
