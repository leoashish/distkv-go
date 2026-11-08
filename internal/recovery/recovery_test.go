package recovery_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"distkv/internal/raft"
	"distkv/internal/recovery"
)

// Step 06 -- crash recovery.   Run:  go test ./internal/recovery -run Step06 -race -v

func buildCommittedNode(t *testing.T) *raft.Node {
	t.Helper()
	n := raft.NewNode("solo")
	if n == nil {
		t.Fatal("NewNode returned nil (step 05 not solved yet)")
	}
	n.BecomeLeader()
	for i, cmd := range []string{"set color red", "set size big", "set color blue"} {
		if err := n.Propose(cmd, nil); err != nil {
			t.Fatalf("Propose(%q, %d): %v", cmd, i, err)
		}
	}
	return n
}

func TestStep06_SaveThenLoad_RoundTripsTermLogCommitIndexAndApplied(t *testing.T) {
	n := buildCommittedNode(t)
	path := filepath.Join(t.TempDir(), "node.json")

	if err := recovery.SaveNode(path, n); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	loaded, err := recovery.LoadNode(path)
	if err != nil {
		t.Fatalf("LoadNode: %v", err)
	}

	if loaded.Term != n.Term {
		t.Fatalf("loaded Term = %d, want %d", loaded.Term, n.Term)
	}
	if loaded.CommitIndex != n.CommitIndex {
		t.Fatalf("loaded CommitIndex = %d, want %d", loaded.CommitIndex, n.CommitIndex)
	}
	if len(loaded.Log) != len(n.Log) {
		t.Fatalf("loaded log has %d entries, want %d", len(loaded.Log), len(n.Log))
	}
	for i := range n.Log {
		if loaded.Log[i] != n.Log[i] {
			t.Fatalf("loaded entry %d = %+v, want %+v", i, loaded.Log[i], n.Log[i])
		}
	}
	if len(loaded.Applied) != len(n.Applied) {
		t.Fatalf("loaded Applied = %v, want %v", loaded.Applied, n.Applied)
	}
	for i := range n.Applied {
		if loaded.Applied[i] != n.Applied[i] {
			t.Fatalf("loaded Applied[%d] = %q, want %q", i, loaded.Applied[i], n.Applied[i])
		}
	}
}

func TestStep06_Load_ReplaysTheCommittedPrefixIntoTheStore(t *testing.T) {
	n := buildCommittedNode(t)
	path := filepath.Join(t.TempDir(), "node.json")
	if err := recovery.SaveNode(path, n); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	loaded, err := recovery.LoadNode(path)
	if err != nil {
		t.Fatalf("LoadNode: %v", err)
	}

	if got, err := loaded.Store.Get(context.Background(), "color"); err != nil || got != "blue" {
		t.Fatalf("color = %q, %v, want blue (last committed write)", got, err)
	}
	if got, err := loaded.Store.Get(context.Background(), "size"); err != nil || got != "big" {
		t.Fatalf("size = %q, %v, want big", got, err)
	}
}

func TestStep06_Load_MissingFile_IsAnErrorWrappingOsErrNotExist(t *testing.T) {
	_, err := recovery.LoadNode(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("LoadNode(missing) = nil, want an error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadNode(missing) = %v, want an error wrapping os.ErrNotExist", err)
	}
}

func TestStep06_SaveNode_FailsOnUnwritablePath(t *testing.T) {
	n := buildCommittedNode(t)

	err := recovery.SaveNode(filepath.Join(t.TempDir(), "missing-dir", "node.json"), n)
	if err == nil {
		t.Fatal("SaveNode to a missing directory = nil, want an error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("SaveNode to a missing directory = %v, want an error wrapping os.ErrNotExist", err)
	}
}

func TestStep06_Load_RejectsAMissingIndexInTheLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	writeNodeFile(t, path, fileNode{
		ID: "node", Term: 1, CommitIndex: 0,
		Log: []raft.LogEntry{
			{Term: 1, Index: 1, Command: "set a 1"},
			{Term: 1, Index: 3, Command: "set a 3"}, // index 2 missing
		},
	})

	if _, err := recovery.LoadNode(path); !errors.Is(err, recovery.ErrInconsistentLog) {
		t.Fatalf("LoadNode(gap in log) = %v, want recovery.ErrInconsistentLog", err)
	}
}

func TestStep06_Load_RejectsACommitIndexBeyondTheLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	writeNodeFile(t, path, fileNode{
		ID: "node", Term: 1, CommitIndex: 5,
		Log: []raft.LogEntry{
			{Term: 1, Index: 1, Command: "set a 1"},
			{Term: 1, Index: 2, Command: "set a 2"},
		},
	})

	if _, err := recovery.LoadNode(path); !errors.Is(err, recovery.ErrInconsistentLog) {
		t.Fatalf("LoadNode(commit index 5 out of log len 2) = %v, want recovery.ErrInconsistentLog", err)
	}
}

func TestStep06_Load_RejectsAnEntryFromTheFuture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	writeNodeFile(t, path, fileNode{
		ID: "node", Term: 1, CommitIndex: 1,
		Log: []raft.LogEntry{
			{Term: 1, Index: 1, Command: "set a 1"},
			{Term: 7, Index: 2, Command: "set a 7"}, // term from a future no one lives in
		},
		Applied: []string{"set a 1", "set a 7"},
	})

	if _, err := recovery.LoadNode(path); !errors.Is(err, recovery.ErrInconsistentLog) {
		t.Fatalf("LoadNode(entry term 7 > node term 1) = %v, want recovery.ErrInconsistentLog", err)
	}
}

func TestStep06_Load_RejectsAnAppliedListThatLies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	writeNodeFile(t, path, fileNode{
		ID: "node", Term: 1, CommitIndex: 1,
		Log: []raft.LogEntry{
			{Term: 1, Index: 1, Command: "set a 1"},
		},
		Applied: []string{"set a 999"}, // the committed entry is "set a 1"; Applied lies
	})

	if _, err := recovery.LoadNode(path); !errors.Is(err, recovery.ErrInconsistentApplied) {
		t.Fatalf("LoadNode(lying Applied) = %v, want recovery.ErrInconsistentApplied", err)
	}
}

func TestStep06_Load_RestoresTheLogButDoesNotApplyBeyondCommitIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.json")
	writeNodeFile(t, path, fileNode{
		ID: "node", Term: 3, CommitIndex: 1,
		Log: []raft.LogEntry{
			{Term: 3, Index: 1, Command: "set k v1"},
			{Term: 3, Index: 2, Command: "set k UNSAVED"}, // present, never committed
		},
		Applied: []string{"set k v1"},
	})

	loaded, err := recovery.LoadNode(path)
	if err != nil {
		t.Fatalf("LoadNode: %v", err)
	}

	if len(loaded.Log) != 2 {
		t.Fatalf("loaded log len = %d, want 2 (uncommitted entries are kept, just not applied)", len(loaded.Log))
	}
	if got, err := loaded.Store.Get(context.Background(), "k"); err != nil || got != "v1" {
		t.Fatalf("k = %q, %v, want v1 (the only committed value)", got, err)
	}
	if len(loaded.Applied) != 1 {
		t.Fatalf("Applied = %v, want exactly the committed prefix", loaded.Applied)
	}
}

func TestStep06_RecoveredNode_CanBePromotedAndServesAgain(t *testing.T) {
	n := buildCommittedNode(t)
	path := filepath.Join(t.TempDir(), "node.json")
	if err := recovery.SaveNode(path, n); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	loaded, err := recovery.LoadNode(path)
	if err != nil {
		t.Fatalf("LoadNode: %v", err)
	}
	if loaded.Role != raft.Follower {
		t.Fatalf("Role = %v, want raft.Follower (a restarted node is never leader)", loaded.Role)
	}
	if err := loaded.Propose("set z 9", nil); !errors.Is(err, raft.ErrNotLeader) {
		t.Fatalf("propose from a follower = %v, want raft.ErrNotLeader", err)
	}

	loaded.BecomeLeader() // term = saved + 1
	if err := loaded.Propose("set z 9", nil); err != nil {
		t.Fatalf("after promotion, Propose: %v", err)
	}
	if got, err := loaded.Store.Get(context.Background(), "z"); err != nil || got != "9" {
		t.Fatalf("z = %q, %v, want 9", got, err)
	}
}

// fileNode mirrors the on-disk format SaveNode writes; the tests craft raw files against this contract.
type fileNode struct {
	ID          string          `json:"id"`
	Term        uint64          `json:"term"`
	CommitIndex uint64          `json:"commit_index"`
	Log         []raft.LogEntry `json:"log"`
	Applied     []string        `json:"applied"`
}

func writeNodeFile(t *testing.T, path string, f fileNode) {
	t.Helper()
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
