//go:generate go tool mockgen -source=trip_metrics_contract.go -destination=trip_metrics_mocks_test.go -package=telemetry

package telemetry

import "context"

type activeTripCounter interface {
	CountActive(context.Context) (int64, error)
}
