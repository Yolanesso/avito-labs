package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const positionBatchSizeEnv = "POSITION_SCAN_BATCH_SIZE"

type Config struct {
	HTTPAddr        string
	HTTPTimeouts    HTTPTimeouts
	LogLevel        string
	ShutdownTimeout time.Duration
	Database        DatabaseConfig
	Positions       PositionsConfig
}

type HTTPTimeouts struct {
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type PositionsConfig struct {
	CheckInterval  time.Duration
	StaleAfter     time.Duration
	Workers        int
	QueueSize      int
	BatchSize      int
	PushHTTPURL    string
	RequestTimeout time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

func Load() (Config, error) {
	httpAddr, err := required("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}

	logLevel, err := required("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	databaseURL, err := required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	maxConns, err := int32Value("DATABASE_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}

	minConns, err := int32Value("DATABASE_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}

	if minConns > maxConns {
		return Config{}, fmt.Errorf(
			"DATABASE_MIN_CONNS cannot be greater than DATABASE_MAX_CONNS",
		)
	}

	maxConnLifetime, err := duration("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return Config{}, err
	}

	connectTimeout, err := duration("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	queryTimeout, err := duration("DATABASE_QUERY_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	positions, err := loadPositions()
	if err != nil {
		return Config{}, err
	}
	httpTimeouts, err := loadHTTPTimeouts()
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:        httpAddr,
		HTTPTimeouts:    httpTimeouts,
		LogLevel:        logLevel,
		ShutdownTimeout: shutdownTimeout,
		Positions:       positions,
		Database: DatabaseConfig{
			URL:             databaseURL,
			MaxConns:        maxConns,
			MinConns:        minConns,
			MaxConnLifetime: maxConnLifetime,
			ConnectTimeout:  connectTimeout,
			QueryTimeout:    queryTimeout,
		},
	}, nil
}

func loadHTTPTimeouts() (HTTPTimeouts, error) {
	var cfg HTTPTimeouts
	for _, field := range []struct {
		name     string
		fallback string
		target   *time.Duration
	}{
		{"HTTP_READ_TIMEOUT", "10s", &cfg.ReadTimeout},
		{"HTTP_READ_HEADER_TIMEOUT", "5s", &cfg.ReadHeaderTimeout},
		{"HTTP_WRITE_TIMEOUT", "10s", &cfg.WriteTimeout},
		{"HTTP_IDLE_TIMEOUT", "60s", &cfg.IdleTimeout},
	} {
		value, err := time.ParseDuration(envOrDefault(field.name, field.fallback))
		if err != nil || value <= 0 {
			return HTTPTimeouts{}, fmt.Errorf("%s must be a positive duration", field.name)
		}
		*field.target = value
	}
	return cfg, nil
}

func loadPositions() (PositionsConfig, error) {
	var cfg PositionsConfig
	var err error
	for _, field := range []struct {
		name     string
		fallback string
		target   *time.Duration
	}{
		{"POSITION_CHECK_INTERVAL", "30s", &cfg.CheckInterval},
		{"POSITION_STALE_AFTER", "60s", &cfg.StaleAfter},
		{"PUSH_REQUEST_TIMEOUT", "1s", &cfg.RequestTimeout},
	} {
		value := envOrDefault(field.name, field.fallback)
		*field.target, err = time.ParseDuration(value)
		if err != nil || *field.target <= 0 {
			return PositionsConfig{}, fmt.Errorf("%s must be a positive duration", field.name)
		}
	}
	for _, field := range []struct {
		name     string
		fallback string
		target   *int
	}{
		{"POSITION_WORKERS", "4", &cfg.Workers},
		{"POSITION_QUEUE_SIZE", "100", &cfg.QueueSize},
		{positionBatchSizeEnv, "100", &cfg.BatchSize},
	} {
		*field.target, err = strconv.Atoi(envOrDefault(field.name, field.fallback))
		if err != nil || *field.target <= 0 {
			return PositionsConfig{}, fmt.Errorf("%s must be a positive integer", field.name)
		}
	}
	cfg.PushHTTPURL, err = required("PUSH_HTTP_URL")
	if err != nil {
		return PositionsConfig{}, err
	}
	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func required(name string) (string, error) {
	value, exists := os.LookupEnv(name)
	if !exists || value == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}

	return value, nil
}

func duration(name string) (time.Duration, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s has invalid duration %q: %w", name, value, err)
	}

	if result <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}

	return result, nil
}

func int32Value(name string) (int32, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}

	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s has invalid integer %q: %w", name, value, err)
	}

	if result < 0 {
		return 0, fmt.Errorf("%s cannot be negative", name)
	}

	return int32(result), nil
}
