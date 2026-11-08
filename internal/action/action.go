package action

import (
	"booster/internal/model"
	"context"
	"errors"
)

var ErrNotApplicable = errors.New("not applicable on this system")

type Action interface {
	Name() string

	Check(ctx context.Context) (model.Change, error)

	Apply(ctx context.Context) error
}
