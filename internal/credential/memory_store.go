package credential

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu     sync.RWMutex
	values map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{values: make(map[string][]byte)}
}

func (s *MemoryStore) Put(ctx context.Context, value []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	reference, err := newReference()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.values[reference] = append([]byte(nil), value...)
	s.mu.Unlock()
	return reference, nil
}

func (s *MemoryStore) Resolve(ctx context.Context, reference string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[reference]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (s *MemoryStore) Delete(ctx context.Context, reference string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.values, reference)
	s.mu.Unlock()
	return nil
}
