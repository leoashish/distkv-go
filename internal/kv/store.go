package kv

import (
	"context"

	"distkv/internal/todo"
)

// STEP 01: implement the store. Read docs/steps/step-01-inmemory-store.md first.   Run:  go test ./internal/kv -run Step01

// Get returns the value stored under key, or an error satisfying errors.Is(err, ErrNotFound).
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	return "", todo.NotImplemented("Step 01: Store.Get")
}

// Put stores value under key, overwriting whatever was there before.
func (s *Store) Put(ctx context.Context, key, value string) error {
	return todo.NotImplemented("Step 01: Store.Put")
}

// Delete removes key. Deleting a key that is not there is a no-op (idempotent).
func (s *Store) Delete(ctx context.Context, key string) error {
	return todo.NotImplemented("Step 01: Store.Delete")
}
