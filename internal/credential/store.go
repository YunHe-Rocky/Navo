package credential

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("credential not found")

type Store interface {
	Put(ctx context.Context, value []byte) (string, error)
	Resolve(ctx context.Context, reference string) ([]byte, error)
	Delete(ctx context.Context, reference string) error
}
