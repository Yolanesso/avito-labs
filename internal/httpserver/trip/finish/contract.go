//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish_test

package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/model"
)

type tripFinisher interface {
	Finish(context.Context, uuid.UUID) (model.Trip, error)
}
