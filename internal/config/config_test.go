package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

const invalidValue = "invalid"

func TestPositionsConfig(t *testing.T) {
	t.Setenv("PUSH_HTTP_URL", "http://push.example.org")
	t.Setenv("POSITION_CHECK_INTERVAL", "30s")
	t.Setenv("POSITION_STALE_AFTER", "60s")
	t.Setenv("POSITION_WORKERS", "4")
	t.Setenv("POSITION_QUEUE_SIZE", "100")
	t.Setenv("POSITION_SCAN_BATCH_SIZE", "250")
	t.Setenv("PUSH_REQUEST_TIMEOUT", "1s")
	cfg, err := loadPositions()
	if err != nil || cfg.CheckInterval != 30*time.Second || cfg.StaleAfter != time.Minute ||
		cfg.Workers != 4 || cfg.QueueSize != 100 || cfg.BatchSize != 250 || cfg.RequestTimeout != time.Second {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"POSITION_CHECK_INTERVAL", "0s"},
		{"POSITION_STALE_AFTER", "-1s"},
		{"PUSH_REQUEST_TIMEOUT", "oops"},
		{"POSITION_WORKERS", "0"},
		{"POSITION_QUEUE_SIZE", "-1"},
		{"POSITION_QUEUE_SIZE", invalidValue},
		{positionBatchSizeEnv, "0"},
		{positionBatchSizeEnv, "-1"},
		{positionBatchSizeEnv, invalidValue},
		{"PUSH_HTTP_URL", ""},
	} {
		t.Run(tc.name+"="+tc.value, func(t *testing.T) {
			t.Setenv(tc.name, tc.value)
			if _, err := loadPositions(); err == nil || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("expected error mentioning %s, got %v", tc.name, err)
			}
		})
	}
}

func TestPositionsDefaultBatchSize(t *testing.T) {
	t.Setenv("PUSH_HTTP_URL", "http://push.example.org")
	t.Setenv(positionBatchSizeEnv, "unused")
	if err := os.Unsetenv(positionBatchSizeEnv); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadPositions()
	if err != nil || cfg.BatchSize != 100 {
		t.Fatalf("default batch size = %d, %v", cfg.BatchSize, err)
	}
}

func TestHTTPTimeouts(t *testing.T) {
	fields := []string{"HTTP_READ_TIMEOUT", "HTTP_READ_HEADER_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT"}
	for _, name := range fields {
		t.Setenv(name, "unused")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := loadHTTPTimeouts()
	want := HTTPTimeouts{10 * time.Second, 5 * time.Second, 10 * time.Second, time.Minute}
	if err != nil || cfg != want {
		t.Fatalf("defaults = %+v, %v", cfg, err)
	}
	for _, name := range fields {
		t.Setenv(name, "2s")
	}
	cfg, err = loadHTTPTimeouts()
	want = HTTPTimeouts{2 * time.Second, 2 * time.Second, 2 * time.Second, 2 * time.Second}
	if err != nil || cfg != want {
		t.Fatalf("custom timeouts = %+v, %v", cfg, err)
	}
	for _, name := range fields {
		for _, invalid := range []string{"", "0s", "-1s", invalidValue} {
			t.Run(name+"="+invalid, func(t *testing.T) {
				t.Setenv(name, invalid)
				if _, err := loadHTTPTimeouts(); err == nil || !strings.Contains(err.Error(), name) {
					t.Fatalf("expected error mentioning %s, got %v", name, err)
				}
			})
		}
	}
}
