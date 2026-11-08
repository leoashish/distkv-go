package kv

import (
	"context"
	"fmt"
)

// REFERENCE SOLUTION for Step 01. Try it yourself first!

// Get returns the value stored under key, or an error satisfying errors.Is(err, ErrNotFound).
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("kv: get %q: %w", key, err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return "", fmt.Errorf("kv: get %q: %w", key, ErrNotFound)
	}
	return v, nil
}

// Put stores value under key, overwriting whatever was there before.
func (s *Store) Put(ctx context.Context, key, value string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("kv: put %q: %w", key, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

// Delete removes key. Deleting a key that is not there is a no-op (idempotent).
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("kv: delete %q: %w", key, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}
