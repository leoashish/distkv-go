package raft

import "distkv/internal/todo"

// STEP 05: implement Raft's log replication. Read docs/steps/step-05-log-replication.md first.   Run:  go test ./internal/raft -run Step05

// NewNode returns a follower with Term 0, an empty log and an empty store.
func NewNode(id string) *Node {
	return nil // TODO Step 05
}

// BecomeLeader makes this node the leader of a NEW term: Term is bumped and Role becomes Leader.
func (n *Node) BecomeLeader() {
	// TODO Step 05
}

// AppendEntries is what a follower runs when the leader sends entries (or a heartbeat with LeaderCommit).
//
//   - Reject stale terms outright.
//   - Check the log matches at PrevLogIndex/PrevLogTerm; on mismatch, refuse.
//   - For each incoming entry, truncate the follower's log from the first MISMATCHED position and append the
//     incoming tail. (This is what repairs a diverged follower.)
//   - Advance CommitIndex to min(LeaderCommit, len(log)) and apply every newly committed entry to the Store
//     ("set key value" strings), recording the command in Applied.
func (n *Node) AppendEntries(req AppendEntriesRequest) AppendEntriesResponse {
	return AppendEntriesResponse{} // TODO Step 05 -- tests fail loudly until you implement this
}

// Propose appends command to the leader's log, replicates it to the followers, and commits it iff a
// majority accepted it (index moves to the entry's index; the entries are applied). Followers learn the
// commit index and apply too. Returns ErrNotLeader if this node is not the leader, ErrNoQuorum otherwise.
func (n *Node) Propose(command string, others []*Node) error {
	return todo.NotImplemented("Step 05: Node.Propose")
}
