package credential

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreCopiesAndDeletesSecrets(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	input := []byte("secret")
	reference, err := store.Put(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	clear(input)
	value, err := store.Resolve(context.Background(), reference)
	if err != nil || string(value) != "secret" {
		t.Fatalf("resolve = %q, %v", value, err)
	}
	clear(value)
	if err := store.Delete(context.Background(), reference); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(context.Background(), reference); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve deleted = %v", err)
	}
}
