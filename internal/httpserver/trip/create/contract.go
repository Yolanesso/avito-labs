//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create_test

package create

import (
	"context"

	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/service"
)

type tripCreator interface {
	Create(context.Context, service.CreateTripInput) (model.Trip, error)
}
