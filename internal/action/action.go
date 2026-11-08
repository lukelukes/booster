package action

import (
	"booster/internal/model"
	"context"
)

type Action interface {
	Name() string

	Check(ctx context.Context) (model.Change, error)

	Apply(ctx context.Context) error
}
