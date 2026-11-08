package replica

import (
	"context"

	"distkv/internal/todo"
)

// STEP 03: implement replication. Read docs/steps/step-03-replication.md first.   Run:  go test ./internal/replica -run Step03

// NewCluster creates a cluster: the first name is the leader, the rest are followers. At least one name is required.
// Majority = (total replicas) / 2 + 1 -- the smallest group that outnumbers the rest.
func NewCluster(names ...string) *Cluster {
	return &Cluster{} // TODO Step 03
}

// Leader returns the cluster's leader. Followers returns a copy of the followers slice.
func (c *Cluster) Leader() *Replica {
	return nil // TODO Step 03
}

func (c *Cluster) Followers() []*Replica {
	return nil // TODO Step 03
}

// Replica.Get reads the value stored on THIS replica (a crash refuses reads). Replica.Set stores the value on THIS replica
// (a crash refuses writes). Replica.Delete removes the key on THIS replica.
func (r *Replica) Get(ctx context.Context, key string) (string, error) {
	return "", todo.NotImplemented("Step 03: Replica.Get")
}

func (r *Replica) Set(ctx context.Context, key, value string) error {
	return todo.NotImplemented("Step 03: Replica.Set")
}

func (r *Replica) Delete(ctx context.Context, key string) error {
	return todo.NotImplemented("Step 03: Replica.Delete")
}

// Put proposes key=value to the cluster: the leader writes first, then every follower. The write COMMITS only if a
// majority (leader included) acknowledged it. If not, every replica that saw the write is rolled back to the pre-write
// state (the key is restored, or deleted if it was not there) and ErrNoQuorum is returned -- a failed write is invisible.
func (c *Cluster) Put(ctx context.Context, key, value string) error {
	return todo.NotImplemented("Step 03: Cluster.Put")
}

// Get reads the value from the LEADER (fresh state). An unknown key returns an error satisfying
// errors.Is(err, kv.ErrNotFound).
func (c *Cluster) Get(ctx context.Context, key string) (string, error) {
	return "", todo.NotImplemented("Step 03: Cluster.Get")
}

// FollowerGet reads the value from the follower at index i. A crashed follower refuses reads.
func (c *Cluster) FollowerGet(ctx context.Context, i int, key string) (string, error) {
	return "", todo.NotImplemented("Step 03: Cluster.FollowerGet")
}

// Crash makes follower i stop serving reads and writes (the failure is sticky until Heal). Heal brings it back --
// but it has MISSED every write made while it was down, and it will not magically re-sync. That divergence is
// the subject of steps 04--05. Out-of-range indices are ignored.
func (c *Cluster) Crash(i int) {
	// TODO Step 03
}

func (c *Cluster) Heal(i int) {
	// TODO Step 03
}
