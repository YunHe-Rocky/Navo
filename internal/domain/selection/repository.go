package selection

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("active selection not found")

type Repository interface {
	Load(ctx context.Context) (ActiveSelection, error)
	Save(ctx context.Context, value ActiveSelection) error
}
