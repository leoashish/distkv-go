package raft

import (
	"context"
	"strings"

	"distkv/internal/kv"
)

// REFERENCE SOLUTION for Step 05. Try it yourself first!

func NewNode(id string) *Node {
	return &Node{ID: id, Role: Follower, Store: kv.NewStore()}
}

func (n *Node) BecomeLeader() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Term++
	n.Role = Leader
}

func (n *Node) AppendEntries(req AppendEntriesRequest) AppendEntriesResponse {
	n.mu.Lock()
	defer n.mu.Unlock()

	if req.Term < n.Term {
		return AppendEntriesResponse{Term: n.Term}
	}
	n.Term = req.Term
	n.Role = Follower

	// Snapshot boundary handling: a node that loaded a snapshot knows nothing before SnapBase. Covered entries are
	// dropped; the snapshot acts as the match position. An exact SnapBase PrevLogIndex must carry SnapTerm.
	if req.PrevLogIndex <= n.SnapBase {
		// Drop entries the snapshot already applied.
		for len(req.Entries) > 0 && req.Entries[0].Index <= n.SnapBase {
			req.Entries = req.Entries[1:]
		}
		if req.PrevLogIndex == n.SnapBase && req.PrevLogTerm != n.SnapTerm {
			return AppendEntriesResponse{Term: n.Term}
		}
	} else {
		idx := req.PrevLogIndex - n.SnapBase - 1 // absolute → 0-based position in the suffix
		if uint64(len(n.Log)) <= idx || n.Log[idx].Term != req.PrevLogTerm {
			return AppendEntriesResponse{Term: n.Term}
		}
	}

	// Append/overwrite incoming entries: truncate the follower's conflicting suffix, then take the tail.
	for _, e := range req.Entries {
		if e.Index <= n.SnapBase {
			continue // already applied in the snapshot
		}
		off := e.Index - n.SnapBase - 1
		if uint64(len(n.Log)) > off && n.Log[off].Term != e.Term {
			n.Log = n.Log[:off]
		}
		if uint64(len(n.Log)) == off {
			n.Log = append(n.Log, e)
		}
	}

	// Commit and apply whatever the leader has committed up to (indices stay absolute).
	if req.LeaderCommit > n.CommitIndex {
		n.CommitIndex = min(req.LeaderCommit, n.lastKnownIndex())
		n.applyLocked()
	}
	return AppendEntriesResponse{Term: n.Term, Success: true}
}

func (n *Node) Propose(command string, others []*Node) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.Role != Leader {
		return ErrNotLeader
	}

	// The new entry's index is an absolute log position: after a snapshot it is NOT len(log)+1.
	next := n.CommitIndex + 1
	if len(n.Log) > 0 {
		if last := n.Log[len(n.Log)-1].Index; last >= next {
			next = last + 1
		}
	}
	entry := LogEntry{Term: n.Term, Index: next, Command: command}
	n.Log = append(n.Log, entry)

	majority := (len(others)+1)/2 + 1
	acks := 1 // the leader
	for _, p := range others {
		prevIndex, prevTerm := entry.Index-1, n.termAtLocked(entry.Index-1)
		resp := p.AppendEntries(AppendEntriesRequest{
			Term:         n.Term,
			LeaderID:     n.ID,
			PrevLogIndex: prevIndex,
			PrevLogTerm:  prevTerm,
			Entries:      []LogEntry{entry},
			LeaderCommit: n.CommitIndex,
		})
		if resp.Success {
			acks++
		}
	}

	if acks < majority {
		return ErrNoQuorum
	}

	n.CommitIndex = entry.Index
	n.applyLocked()

	// Heartbeat round: tell the followers what just committed, so they apply too.
	prevTerm := n.termAtLocked(n.CommitIndex)
	for _, p := range others {
		p.AppendEntries(AppendEntriesRequest{
			Term:         n.Term,
			LeaderID:     n.ID,
			PrevLogIndex: n.CommitIndex,
			PrevLogTerm:  prevTerm,
			Entries:      nil,
			LeaderCommit: n.CommitIndex,
		})
	}
	return nil
}

func (n *Node) termAtLocked(index uint64) uint64 {
	if index == 0 {
		return 0
	}
	if index == n.SnapBase {
		return n.SnapTerm
	}
	if index > n.SnapBase {
		off := index - n.SnapBase - 1
		if off >= uint64(len(n.Log)) {
			return 0
		}
		return n.Log[off].Term
	}
	return 0 // terms of snapshot-covered entries are not reachable (the snapshot is the boundary)
}

func (n *Node) lastKnownIndex() uint64 {
	if len(n.Log) > 0 {
		return n.Log[len(n.Log)-1].Index
	}
	return n.SnapBase // the highest index we know for sure
}

func (n *Node) applyLocked() {
	for n.LastApplied < n.CommitIndex {
		n.LastApplied++
		if n.LastApplied <= n.SnapBase {
			continue // already applied in the snapshot
		}
		entry := n.Log[n.LastApplied-n.SnapBase-1]
		n.Applied = append(n.Applied, entry.Command)
		if parts := strings.Fields(entry.Command); len(parts) == 3 && parts[0] == "set" {
			_ = n.Store.Put(context.Background(), parts[1], parts[2])
		}
	}
}
