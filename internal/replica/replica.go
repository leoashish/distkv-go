// Package replica implements the first honest step of replication: one leader, many followers, and a write
// that only "commits" when a majority acknowledges it.
package replica

import (
	"errors"

	"distkv/internal/kv"
)

var (
	// ErrNoQuorum: fewer than a majority of replicas acknowledged the write. The write was rolled back and is not visible anywhere. GIVEN.
	ErrNoQuorum = errors.New("replica: quorum not reached")
	// ErrReplicaDown: the replica has crashed and refuses reads and writes until it heals. GIVEN.
	ErrReplicaDown = errors.New("replica: the replica is down")
	// ErrNoFollower: no follower exists at the requested index. GIVEN.
	ErrNoFollower = errors.New("replica: no follower at that index")
)

// Role identifies what a replica is for.
type Role int

const (
	RoleFollower Role = iota
	RoleLeader
)

// Replica is one replica of the KV store: a name, a role, an in-memory store (Step 01) and a health flag.
type Replica struct {
	Name    string
	Role    Role
	store   *kv.Store
	healthy bool
}

// Cluster is one replicated store: a leader it accepts writes, followers it replicates to, and the majority
// needed to acknowledge a write.
type Cluster struct {
	leader    *Replica
	followers []*Replica
	majority  int
}
