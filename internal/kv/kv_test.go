package kv_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"distkv/internal/kv"
)

// Step 01 -- the single-node store. Run:  go test ./internal/kv -run Step01 -v

func newStore() *kv.Store { return kv.NewStore() }

func TestStep01_Put_Then_Get_ReturnsTheValue(t *testing.T) {
	s := newStore()

	if err := s.Put(context.Background(), "greeting", "hello, distributed world"); err != nil {
		t.Fatalf("Put: unexpected error: %v", err)
	}

	got, err := s.Get(context.Background(), "greeting")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got != "hello, distributed world" {
		t.Fatalf("Get(%q) = %q, want %q", "greeting", got, "hello, distributed world")
	}
}

func TestStep01_Get_ReturnsErrNotFound_ForAnUnknownKey(t *testing.T) {
	s := newStore()

	_, err := s.Get(context.Background(), "missing")
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("Get(%q) error = %v, want an error satisfying errors.Is(err, kv.ErrNotFound)", "missing", err)
	}
}

func TestStep01_Delete_RemovesTheKey(t *testing.T) {
	s := newStore()

	if err := s.Put(context.Background(), "session:1", "abc"); err != nil {
		t.Fatalf("Put: unexpected error: %v", err)
	}
	if err := s.Delete(context.Background(), "session:1"); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}

	if _, err := s.Get(context.Background(), "session:1"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("after Delete, Get should miss: %v", err)
	}
}

func TestStep01_Delete_IsIdempotent_WhenTheKeyIsMissing(t *testing.T) {
	s := newStore()

	if err := s.Delete(context.Background(), "never-there"); err != nil {
		t.Fatalf("Delete of a missing key should be a no-op, got %v", err)
	}
	if err := s.Delete(context.Background(), "never-there"); err != nil {
		t.Fatalf("Delete of a missing key should be a no-op the second time too, got %v", err)
	}
}

func TestStep01_EveryMethod_ReturnsContextError_WhenContextIsAlreadyCancelled(t *testing.T) {
	s := newStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := s.Put(ctx, "k", "v"); err == nil {
		t.Error("Put with an already-cancelled context: expected an error, got nil")
	}
	if err := s.Delete(ctx, "k"); err == nil {
		t.Error("Delete with an already-cancelled context: expected an error, got nil")
	}
	if _, err := s.Get(ctx, "k"); !errors.Is(err, context.Canceled) {
		t.Errorf("Get with an already-cancelled context returned %v, want an error satisfying errors.Is(err, context.Canceled)", err)
	}
}

func TestStep01_ParallelWritesAndReads_AllSucceed(t *testing.T) {
	s := newStore()
	const n = 20
	errs := make([]error, n)
	values := make([]string, n)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			if err := s.Put(context.Background(), key, "v"); err != nil {
				errs[i] = err
				return
			}
			v, err := s.Get(context.Background(), key)
			values[i] = v
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("op %d: %v", i, errs[i])
		}
		if values[i] != "v" {
			t.Fatalf("op %d: value = %q, want %q", i, values[i], "v")
		}
	}
}
