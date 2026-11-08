package replica_test

import (
	"context"
	"errors"
	"testing"

	"distkv/internal/kv"
	"distkv/internal/replica"
)

// Step 03 -- replication.   Run:  go test ./internal/replica -run Step03 -v

func mustCluster(t *testing.T, names ...string) *replica.Cluster {
	t.Helper()
	cl := replica.NewCluster(names...)
	if cl == nil {
		t.Fatal("NewCluster returned nil")
	}
	if cl.Leader() == nil {
		t.Fatal("NewCluster created no leader")
	}
	if got := len(cl.Followers()); got != len(names)-1 {
		t.Fatalf("Followers = %d, want %d", got, len(names)-1)
	}
	return cl
}

func TestStep03_NewCluster_NamesTheFirstReplicaAsLeader(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")

	if cl.Leader().Name != "alpha" {
		t.Fatalf("leader = %q, want alpha", cl.Leader().Name)
	}
	if cl.Leader().Role != replica.RoleLeader {
		t.Fatalf("leader Role = %v, want replica.RoleLeader", cl.Leader().Role)
	}
	for i, f := range cl.Followers() {
		if f.Role != replica.RoleFollower {
			t.Fatalf("follower %d Role = %v, want replica.RoleFollower", i, f.Role)
		}
	}
}

func TestStep03_PutThenGet_ReturnsTheValueFromTheLeader(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	if err := cl.Put(ctx, "key", "value"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := cl.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "value" {
		t.Fatalf("Get = %q, want %q", got, "value")
	}
}

func TestStep03_Put_ReplicatesToAllHealthyFollowers(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	if err := cl.Put(ctx, "shared", "secret"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	for i := range cl.Followers() {
		got, err := cl.FollowerGet(ctx, i, "shared")
		if err != nil {
			t.Fatalf("FollowerGet(%d): %v", i, err)
		}
		if got != "secret" {
			t.Fatalf("follower %d = %q, want %q", i, got, "secret")
		}
	}
}

func TestStep03_Put_WithOneFollowerCrashed_StillCommits(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	cl.Crash(0)

	if err := cl.Put(ctx, "k", "v"); err != nil {
		t.Fatalf("Put with one follower crashed: %v", err)
	}
	got, err := cl.FollowerGet(ctx, 1, "k")
	if err != nil {
		t.Fatalf("FollowerGet(1): %v", err)
	}
	if got != "v" {
		t.Fatalf("follower 1 = %q, want %q", got, "v")
	}
}

func TestStep03_Put_WithAllFollowersCrashed_FailsAndRollsBack(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	cl.Crash(0)
	cl.Crash(1)

	if err := cl.Put(ctx, "k", "v"); !errors.Is(err, replica.ErrNoQuorum) {
		t.Fatalf("Put with both followers crashed = %v, want replica.ErrNoQuorum", err)
	}
	if _, err := cl.Get(ctx, "k"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("after a rolled-back write, leader Get = %v, want kv.ErrNotFound", err)
	}
}

func TestStep03_Put_WithAllFollowersCrashed_RollsBackAnExistingValueToo(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	if err := cl.Put(ctx, "k", "original"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	cl.Crash(0)
	cl.Crash(1)

	if err := cl.Put(ctx, "k", "doomed"); !errors.Is(err, replica.ErrNoQuorum) {
		t.Fatalf("Put = %v, want replica.ErrNoQuorum", err)
	}
	got, err := cl.Get(ctx, "k")
	if err != nil {
		t.Fatalf("after a rolled-back write, leader Get = %v", err)
	}
	if got != "original" {
		t.Fatalf("after rollback, leader Get = %q, want %q", got, "original")
	}
}

func TestStep03_CrashedFollower_MissesTheWrite_AndNeverCatchesUpAlone(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	if err := cl.Put(ctx, "k", "before"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	cl.Crash(0)
	if err := cl.Put(ctx, "k", "after"); err != nil {
		t.Fatalf("Put with follower 0 crashed (a majority): %v", err)
	}
	cl.Heal(0)

	got, err := cl.Get(ctx, "k")
	if err != nil {
		t.Fatalf("leader Get: %v", err)
	}
	if got != "after" {
		t.Fatalf("leader Get = %q, want %q", got, "after")
	}

	stale, err := cl.FollowerGet(ctx, 0, "k")
	if err != nil {
		t.Fatalf("FollowerGet(0): %v", err)
	}
	if stale == "after" {
		t.Fatal("the healed follower has the value without ever receiving it: replication does not work that way")
	}
	if stale != "before" {
		t.Fatalf("follower 0 = %q, want %q (permanently diverged without consensus)", stale, "before")
	}
}

func TestStep03_Put_Overwrites_TheLatestValue(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	_ = cl.Put(ctx, "k", "first")
	_ = cl.Put(ctx, "k", "second")

	got, err := cl.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "second" {
		t.Fatalf("Get = %q, want %q", got, "second")
	}
}

func TestStep03_Get_BeforeAnyPut_ReturnsErrNotFound(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")

	if _, err := cl.Get(context.Background(), "missing"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("Get before any Put = %v, want an error satisfying errors.Is(err, kv.ErrNotFound)", err)
	}
}

func TestStep03_FollowerGet_WithAnOutOfRangeIndex_ReturnsErrNoFollower(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")

	_, err := cl.FollowerGet(context.Background(), 9, "k")
	if !errors.Is(err, replica.ErrNoFollower) {
		t.Fatalf("FollowerGet(9) = %v, want replica.ErrNoFollower", err)
	}
}

func TestStep03_CrashAndHeal_AreSymmetric_BackToAcceptingWrites(t *testing.T) {
	cl := mustCluster(t, "alpha", "beta", "gamma")
	ctx := context.Background()

	cl.Crash(0)
	cl.Heal(0)

	if err := cl.Put(ctx, "k", "again"); err != nil {
		t.Fatalf("Put after heal: %v", err)
	}
	got, err := cl.FollowerGet(ctx, 0, "k")
	if err != nil {
		t.Fatalf("FollowerGet(0): %v", err)
	}
	if got != "again" {
		t.Fatalf("healed follower = %q, want %q", got, "again")
	}
}
