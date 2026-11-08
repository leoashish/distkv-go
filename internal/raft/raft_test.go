package raft_test

import (
	"context"
	"errors"
	"testing"

	"distkv/internal/kv"
	"distkv/internal/raft"
)

// Step 05 -- Raft: log replication and the commit index.   Run:  go test ./internal/raft -run Step05 -v

func newNode(t *testing.T, id string) *raft.Node {
	t.Helper()
	n := raft.NewNode(id)
	if n == nil {
		t.Fatalf("NewNode(%q) returned nil", id)
	}
	return n
}

func TestStep05_NewNode_StartsAsFollowerAtTermZero(t *testing.T) {
	n := newNode(t, "solo")

	if n.Term != 0 {
		t.Fatalf("Term = %d, want 0", n.Term)
	}
	if n.Role != raft.Follower {
		t.Fatalf("Role = %v, want raft.Follower", n.Role)
	}
	if len(n.Log) != 0 {
		t.Fatalf("Log = %v, want empty", n.Log)
	}
	if n.Store == nil {
		t.Fatal("Store is nil: every node owns a state machine (its own KV store)")
	}
}

func TestStep05_BecomeLeader_MakesTheLeaderOfANewTerm(t *testing.T) {
	n := newNode(t, "node")
	n.BecomeLeader()

	if n.Role != raft.Leader {
		t.Fatalf("Role = %v, want raft.Leader", n.Role)
	}
	if n.Term != 1 {
		t.Fatalf("Term = %d, want 1 (a new term for the new leader)", n.Term)
	}
}

func TestStep05_SingleNode_ProposeCommitsAndApplies(t *testing.T) {
	n := newNode(t, "solo")
	n.BecomeLeader()

	if err := n.Propose("set color red", nil); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if n.CommitIndex != 1 {
		t.Fatalf("CommitIndex = %d, want 1", n.CommitIndex)
	}
	got, err := n.Store.Get(context.Background(), "color")
	if err != nil {
		t.Fatalf("Store.Get: %v", err)
	}
	if got != "red" {
		t.Fatalf("color = %q, want %q", got, "red")
	}
	if len(n.Applied) != 1 || n.Applied[0] != "set color red" {
		t.Fatalf("Applied = %v, want [set color red]", n.Applied)
	}
}

func TestStep05_ThreeNodes_ProposeReplicatesAndAppliesEverywhere(t *testing.T) {
	leader := newNode(t, "leader")
	f1, f2 := newNode(t, "f1"), newNode(t, "f2")
	leader.BecomeLeader()

	if err := leader.Propose("set k v", []*raft.Node{f1, f2}); err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if leader.CommitIndex != 1 {
		t.Fatalf("leader CommitIndex = %d, want 1", leader.CommitIndex)
	}
	for i, f := range []*raft.Node{f1, f2} {
		if len(f.Log) != 1 {
			t.Fatalf("follower %d log len = %d, want 1", i, len(f.Log))
		}
		if f.CommitIndex != 1 {
			t.Fatalf("follower %d CommitIndex = %d, want 1 (the heartbeat round teaches them)", i, f.CommitIndex)
		}
		got, err := f.Store.Get(context.Background(), "k")
		if err != nil {
			t.Fatalf("follower %d Store.Get: %v", i, err)
		}
		if got != "v" {
			t.Fatalf("follower %d k = %q, want %q", i, got, "v")
		}
	}
}

func TestStep05_FollowerDoesNotCommitOrApply_UntilTheLeaderCommits(t *testing.T) {
	f := newNode(t, "f")
	req := raft.AppendEntriesRequest{
		Term:         1,
		LeaderID:     "leader",
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      []raft.LogEntry{{Term: 1, Index: 1, Command: "set a b"}},
		LeaderCommit: 0, // not committed yet
	}
	if resp := f.AppendEntries(req); !resp.Success {
		t.Fatal("follower must accept the entry (it just cannot be committed yet)")
	}

	if f.CommitIndex != 0 {
		t.Fatalf("follower CommitIndex = %d, want 0 before the leader commits", f.CommitIndex)
	}
	if _, err := f.Store.Get(context.Background(), "a"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("before commit, follower store.Get = %v, want kv.ErrNotFound", err)
	}

	commit := raft.AppendEntriesRequest{Term: 1, LeaderID: "leader", PrevLogIndex: 1, PrevLogTerm: 1, Entries: nil, LeaderCommit: 1}
	if resp := f.AppendEntries(commit); !resp.Success {
		t.Fatal("follower must accept the heartbeat with LeaderCommit")
	}
	if f.CommitIndex != 1 {
		t.Fatalf("follower CommitIndex = %d, want 1 after the leader committed", f.CommitIndex)
	}
	if got := mustGet(t, f, "a"); got != "b" {
		t.Fatalf("a = %q, want %q", got, "b")
	}
}

func TestStep05_DivergedFollowerEntryIsTruncated_AndRepaired(t *testing.T) {
	leader := newNode(t, "leader")
	f := newNode(t, "f")
	f.Term = 1 // the follower has an older-term bogus entry at index 1 from somewhere else
	f.Log = []raft.LogEntry{{Term: 1, Index: 1, Command: "set k bogus"}}

	leader.BecomeLeader() // term 1
	leader.BecomeLeader() // term 2: a NEW leader in a NEWER term

	if err := leader.Propose("set k v2", []*raft.Node{f}); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if len(f.Log) != 1 || f.Log[0].Command != "set k v2" {
		t.Fatalf("follower log = %v, want the bogus entry truncated and replaced with the leader's", f.Log)
	}
	if got := mustGet(t, f, "k"); got != "v2" {
		t.Fatalf("k = %q, want %q", got, "v2")
	}
}

func TestStep05_ProposeWithoutMajority_DoesNotCommit_ButTheEntrySurvivesInTheLog(t *testing.T) {
	leader := newNode(t, "leader")
	f1, f2, f3, f4 := newNode(t, "f1"), newNode(t, "f2"), newNode(t, "f3"), newNode(t, "f4")
	f2.Term = 99
	f3.Term = 99
	f4.Term = 99 // only f1 will accept

	leader.BecomeLeader()
	err := leader.Propose("set k v", []*raft.Node{f1, f2, f3, f4})

	if !errors.Is(err, raft.ErrNoQuorum) {
		t.Fatalf("Propose = %v, want raft.ErrNoQuorum", err)
	}
	if leader.CommitIndex != 0 {
		t.Fatalf("CommitIndex = %d, want 0 (no majority means no commit)", leader.CommitIndex)
	}
	if len(leader.Log) != 1 {
		t.Fatalf("leader log = %v, want the uncommitted entry to remain (it may commit later)", leader.Log)
	}
	if got := mustGetOrNotFound(t, leader, "k"); got != "" {
		t.Fatalf("k = %q, want not-found (nothing committed)", got)
	}
}

func TestStep05_Propose_FromAFollower_ReturnsErrNotLeader(t *testing.T) {
	f := newNode(t, "f")

	err := f.Propose("set k v", nil)
	if !errors.Is(err, raft.ErrNotLeader) {
		t.Fatalf("Propose = %v, want raft.ErrNotLeader", err)
	}
}

func TestStep05_AppliedCommandsAreInOrder_AndLaterWritesWin(t *testing.T) {
	n := newNode(t, "solo")
	n.BecomeLeader()

	for _, cmd := range []string{"set a 1", "set b 2", "set a 3"} {
		if err := n.Propose(cmd, nil); err != nil {
			t.Fatalf("Propose(%q): %v", cmd, err)
		}
	}

	want := []string{"set a 1", "set b 2", "set a 3"}
	if len(n.Applied) != len(want) {
		t.Fatalf("Applied = %v, want %v", n.Applied, want)
	}
	for i := range want {
		if n.Applied[i] != want[i] {
			t.Fatalf("Applied[%d] = %q, want %q", i, n.Applied[i], want[i])
		}
	}
	if got := mustGet(t, n, "a"); got != "3" {
		t.Fatalf("a = %q, want %q", got, "3")
	}
}

func TestStep05_LogIndicesAreOneBasedAndContiguous(t *testing.T) {
	n := newNode(t, "solo")
	n.BecomeLeader()

	for _, cmd := range []string{"set x 1", "set y 2", "set z 3"} {
		if err := n.Propose(cmd, nil); err != nil {
			t.Fatalf("Propose(%q): %v", cmd, err)
		}
	}
	for i, entry := range n.Log {
		if entry.Index != uint64(i+1) {
			t.Fatalf("entry %d has index %d, want %d", i, entry.Index, i+1)
		}
		if entry.Term != n.Term {
			t.Fatalf("entry %d has term %d, want the leader's term %d", i, entry.Term, n.Term)
		}
	}
	if n.CommitIndex != 3 {
		t.Fatalf("CommitIndex = %d, want 3", n.CommitIndex)
	}
}

func mustGet(t *testing.T, n *raft.Node, key string) string {
	t.Helper()
	got, err := n.Store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Store.Get(%q): %v", key, err)
	}
	return got
}

func mustGetOrNotFound(t *testing.T, n *raft.Node, key string) string {
	t.Helper()
	got, err := n.Store.Get(context.Background(), key)
	if errors.Is(err, kv.ErrNotFound) {
		return ""
	}
	if err != nil {
		t.Fatalf("Store.Get(%q): %v", key, err)
	}
	return got
}
