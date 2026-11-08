// Package kv is DistKV's key-value store. Step 01 gives a DistKV node its brain: one in-memory map.
package kv

import (
	"errors"
	"sync"
)

// ErrNotFound is returned (possibly wrapped) by Get when the key is not there.
// Check for it with errors.Is(err, kv.ErrNotFound).
var ErrNotFound = errors.New("not found")

// Store holds the key-value pairs owned by a single DistKV node.
//
// STEP 01: a single node is just an in-memory map behind a RWMutex. Goroutines (HTTP handlers, background
// workers, tests) may call Get/Put/Delete concurrently; RLock lets many readers run at once, Lock serialises
// writes. Later steps put a write-ahead log in front of this map, then add a leader role, followers, elections
// and finally a replicated log around it.
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewStore returns a fresh, empty store: a brand-new node that has never seen a key.
func NewStore() *Store {
	return &Store{data: make(map[string]string)}
}
