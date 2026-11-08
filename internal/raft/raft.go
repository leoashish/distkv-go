// Package raft implements the heart of Raft in miniature: a replicated log with a commit index.
// The leader owns the log; followers copy it; entries become visible the moment a majority holds them.
package raft

import (
	"errors"
	"sync"

	"distkv/internal/kv"
)

var (
	// ErrNotLeader: only the leader of a term may append entries. GIVEN.
	ErrNotLeader = errors.New("raft: only the leader may propose")
	// ErrNoQuorum: fewer than a majority of nodes accepted the entry. It is NOT committed. GIVEN.
	ErrNoQuorum = errors.New("raft: quorum not reached: the entry is not committed")
)

// Role is what a node is in its term.
type Role int

const (
	Follower Role = iota
	Candidate
	Leader
)

// LogEntry is one command in the replicated log. Index is 1-based and contiguous; Term is the
// leader's term when the entry was created.
type LogEntry struct {
	Term    uint64
	Index   uint64
	Command string // "set <key> <value>"
}

// AppendEntriesRequest is what the leader sends to a follower.
type AppendEntriesRequest struct {
	Term         uint64
	LeaderID     string
	PrevLogIndex uint64 // 0 = no previous entry
	PrevLogTerm  uint64
	Entries      []LogEntry
	LeaderCommit uint64
}

// AppendEntriesResponse is the follower's answer.
type AppendEntriesResponse struct {
	Term    uint64
	Success bool
}

// Node is one replica in a Raft cluster.
type Node struct {
	ID      string
	Term    uint64
	Role    Role
	Log     []LogEntry
	Store   *kv.Store // the applied state machine
	Applied []string  // the commands applied, in order

	CommitIndex uint64

	// SnapBase / SnapTerm mark the boundary of a snapshot: entries with index <= SnapBase were already applied before
	// this node loaded its state, are not in Log, and their terms are not reachable except for SnapTerm itself.
	// A fresh node has SnapBase = 0 (no snapshot). Set by a snapshot loader, never reordered otherwise.
	SnapBase uint64
	SnapTerm uint64

	mu          sync.Mutex
	LastApplied uint64 // the index of the last entry applied to the Store (the "apply cursor"); a snapshot sets it explicitly
	votedFor    string
}
