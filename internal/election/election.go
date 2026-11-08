// Package election implements Raft-style leader election: terms, votes and heartbeats — the
// part of consensus that makes a stale leader visible.
package election

// State is what a node currently is.
type State int

const (
	Follower State = iota
	Candidate
	Leader
)

// VoteRequest is what a candidate sends every peer when it calls an election.
type VoteRequest struct {
	Term        uint64
	CandidateID string
}

// VoteResponse is the peer's decision.
type VoteResponse struct {
	// Term is the voter's term after handling the request. A stale candidate learns of a newer term and must step down.
	Term uint64
	// VoteGranted is true iff the voter granted the vote.
	VoteGranted bool
}

// Node is one voting peer: a term, at most one vote per term, and a state machine
// (Follower → Candidate → Leader).
type Node struct {
	ID    string
	Term  uint64
	State State
	// votedFor is the ID this node voted for in the CURRENT term ("" for none). Reset when the term moves on.
	votedFor string
}
