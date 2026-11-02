package telemetry

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func NewLogger(level string) (*slog.Logger, error) {
	var logLevel slog.Level

	err := logLevel.UnmarshalText([]byte(strings.ToUpper(level)))
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}

	logger := slog.New(slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: logLevel},
	))

	return logger.With("service", "trip-service"), nil
}
