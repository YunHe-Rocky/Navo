package credential

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"navo/internal/securestore"
)

type FileStore struct {
	path   string
	mu     sync.RWMutex
	values map[string][]byte
}

func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("credential store path is required")
	}
	store := &FileStore{path: path, values: make(map[string][]byte)}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStore) Put(ctx context.Context, value []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(value) == 0 {
		return "", fmt.Errorf("credential value is empty")
	}
	reference, err := newReference()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[reference] = append([]byte(nil), value...)
	if err := s.saveLocked(); err != nil {
		delete(s.values, reference)
		return "", err
	}
	return reference, nil
}

func (s *FileStore) Resolve(ctx context.Context, reference string) ([]byte, error) {
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

func (s *FileStore) Delete(ctx context.Context, reference string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, ok := s.values[reference]
	if !ok {
		return nil
	}
	delete(s.values, reference)
	if err := s.saveLocked(); err != nil {
		s.values[reference] = previous
		return err
	}
	return nil
}

func (s *FileStore) load() error {
	encrypted, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read credential store: %w", err)
	}
	plain, err := securestore.Unprotect(encrypted)
	if err != nil {
		return fmt.Errorf("decrypt credential store: %w", err)
	}
	defer clear(plain)
	if err := json.Unmarshal(plain, &s.values); err != nil {
		return fmt.Errorf("decode credential store: %w", err)
	}
	return nil
}

func (s *FileStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	plain, err := json.Marshal(s.values)
	if err != nil {
		return fmt.Errorf("encode credential store: %w", err)
	}
	defer clear(plain)
	encrypted, err := securestore.Protect(plain)
	if err != nil {
		return fmt.Errorf("encrypt credential store: %w", err)
	}
	defer clear(encrypted)

	temp, err := os.CreateTemp(filepath.Dir(s.path), ".credentials-*")
	if err != nil {
		return fmt.Errorf("create credential temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(encrypted); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("activate credential store: %w", err)
	}
	return nil
}

func newReference() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate credential reference: %w", err)
	}
	return "credential://" + hex.EncodeToString(random), nil
}
