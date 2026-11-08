package snapshot_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"distkv/internal/kv"
	"distkv/internal/raft"
	"distkv/internal/snapshot"
)

// Step 07 -- snapshots and suffix compaction.   Run:  go test ./internal/snapshot -run Step07 -v

func newNode(t *testing.T, id string) *raft.Node {
	t.Helper()
	n := raft.NewNode(id)
	if n == nil {
		t.Fatal("NewNode returned nil (step 05 not solved yet)")
	}
	return n
}

func buildCommittedNode(t *testing.T) *raft.Node {
	t.Helper()
	n := newNode(t, "solo")
	n.BecomeLeader()
	for _, cmd := range []string{"set color red", "set size big", "set color blue"} {
		if err := n.Propose(cmd, nil); err != nil {
			t.Fatalf("Propose(%q): %v", cmd, err)
		}
	}
	return n
}

func TestStep07_CompactThenLoad_RoundTripsTermCommitIndexAppliedAndLog(t *testing.T) {
	n := buildCommittedNode(t)
	path := filepath.Join(t.TempDir(), "snapshot.json")

	if err := snapshot.Compact(path, n); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	loaded, err := snapshot.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Term != n.Term {
		t.Fatalf("Term = %d, want %d", loaded.Term, n.Term)
	}
	if loaded.CommitIndex != n.CommitIndex {
		t.Fatalf("CommitIndex = %d, want %d", loaded.CommitIndex, n.CommitIndex)
	}
	if len(loaded.Applied) != len(n.Applied) {
		t.Fatalf("Applied = %v, want %v", loaded.Applied, n.Applied)
	}
	for i := range n.Applied {
		if loaded.Applied[i] != n.Applied[i] {
			t.Fatalf("Applied[%d] = %q, want %q", i, loaded.Applied[i], n.Applied[i])
		}
	}
}

func TestStep07_Load_RestoresTheCompleteStoreState(t *testing.T) {
	n := buildCommittedNode(t)
	path := filepath.Join(t.TempDir(), "snapshot.json")

	if err := snapshot.Compact(path, n); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	loaded, err := snapshot.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for key, want := range map[string]string{"color": "blue", "size": "big"} {
		got, err := loaded.Store.Get(context.Background(), key)
		if err != nil || got != want {
			t.Fatalf("loaded store[%q] = %q, %v, want %q", key, got, err, want)
		}
	}
}

func TestStep07_Load_KeepsAnUncommittedSuffix_WithoutApplyingIt(t *testing.T) {
	n := buildCommittedNode(t)           // log length 3, CommitIndex 3
	n.Log = append(n.Log, raft.LogEntry{ // craft an uncommitted tail
		Term:    n.Term,
		Index:   4,
		Command: "set ghost uncommitted",
	})
	path := filepath.Join(t.TempDir(), "snapshot.json")

	if err := snapshot.Compact(path, n); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	loaded, err := snapshot.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Log) != 1 {
		t.Fatalf("loaded log len = %d, want 1 (the suffix)", len(loaded.Log))
	}
	if loaded.Log[0].Index != 4 {
		t.Fatalf("suffix index = %d, want 4", loaded.Log[0].Index)
	}
	if loaded.CommitIndex != 3 {
		t.Fatalf("CommitIndex = %d, want 3 (uncommitted suffix is never applied)", loaded.CommitIndex)
	}
	if _, err := loaded.Store.Get(context.Background(), "ghost"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("the uncommitted suffix must not reach the store, got %v", err)
	}
	if len(loaded.Applied) != 3 {
		t.Fatalf("Applied = %v, want 3 committed entries", loaded.Applied)
	}
}

func TestStep07_Compact_IsAtomic_AndLeavesNoTempFile(t *testing.T) {
	n := buildCommittedNode(t)
	path := filepath.Join(t.TempDir(), "snapshot.json")

	if err := snapshot.Compact(path, n); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the snapshot was not saved: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("a temp file was left behind: the rename must have consumed it")
	}
}

func TestStep07_Load_MissingFile_IsAnErrorWrappingOsErrNotExist(t *testing.T) {
	_, err := snapshot.Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("Load(missing) = nil, want an error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load(missing) = %v, want an error wrapping os.ErrNotExist", err)
	}
}

func TestStep07_Load_RejectsASuffixWithAGap(t *testing.T) {
	path := writeFileSnapshot(t, fileSnapshot{
		ID: "node", Term: 1, CommitIndex: 1,
		Applied: []string{"set a 1"},
		Data:    map[string]string{"a": "1"},
		Suffix:  []raft.LogEntry{{Term: 1, Index: 3, Command: "set a 3"}}, // index 2 missing
	})

	if _, err := snapshot.Load(path); !errors.Is(err, snapshot.ErrInvalidSuffix) {
		t.Fatalf("Load(gap in suffix) = %v, want snapshot.ErrInvalidSuffix", err)
	}
}

func TestStep07_Load_RejectsASuffixEntryFromTheFuture(t *testing.T) {
	path := writeFileSnapshot(t, fileSnapshot{
		ID: "node", Term: 1, CommitIndex: 1,
		Applied: []string{"set a 1"},
		Data:    map[string]string{"a": "1"},
		Suffix:  []raft.LogEntry{{Term: 7, Index: 2, Command: "set a 7"}},
	})

	if _, err := snapshot.Load(path); !errors.Is(err, snapshot.ErrInvalidSuffix) {
		t.Fatalf("Load(entry from future term) = %v, want snapshot.ErrInvalidSuffix", err)
	}
}

func TestStep07_Load_RejectsAnAppliedListOfTheWrongLength(t *testing.T) {
	path := writeFileSnapshot(t, fileSnapshot{
		ID: "node", Term: 1, CommitIndex: 2,
		Applied: []string{"set a 1"}, // commit index 2 but only one applied command
		Data:    map[string]string{"a": "1"},
		Suffix:  nil,
	})

	if _, err := snapshot.Load(path); !errors.Is(err, snapshot.ErrInvalidSnapshot) {
		t.Fatalf("Load(wrong applied length) = %v, want snapshot.ErrInvalidSnapshot", err)
	}
}

func TestStep07_Load_RejectsADataMapThatLiesAboutTheAppliedCommands(t *testing.T) {
	path := writeFileSnapshot(t, fileSnapshot{
		ID: "node", Term: 1, CommitIndex: 1,
		Applied: []string{"set a 1"},
		Data:    map[string]string{"a": "2"}, // applied says 1, data says 2
	})

	if _, err := snapshot.Load(path); !errors.Is(err, snapshot.ErrInvalidSnapshot) {
		t.Fatalf("Load(lying data) = %v, want snapshot.ErrInvalidSnapshot", err)
	}
}

func TestStep07_ReloadedNode_KeepsCommittedState_AndAcceptsMoreCommittedEntries(t *testing.T) {
	n := buildCommittedNode(t) // color=blue, size=big, color red→blue
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := snapshot.Compact(path, n); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	loaded, err := snapshot.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	loaded.BecomeLeader() // term = saved + 1

	if err := loaded.Propose("set z 9", nil); err != nil {
		t.Fatalf("after reload, Propose: %v", err)
	}
	if got, err := loaded.Store.Get(context.Background(), "z"); err != nil || got != "9" {
		t.Fatalf("z = %q, %v, want 9", got, err)
	}
	if got, err := loaded.Store.Get(context.Background(), "color"); err != nil || got != "blue" {
		t.Fatalf("color after reload + propose = %q, %v, want blue", got, err)
	}

	path2 := filepath.Join(dir, "snap2.json")
	if err := snapshot.Compact(path2, loaded); err != nil {
		t.Fatalf("Compact after reload: %v", err)
	}
	final, err := snapshot.Load(path2)
	if err != nil {
		t.Fatalf("final Load: %v", err)
	}
	if got, err := final.Store.Get(context.Background(), "z"); err != nil || got != "9" {
		t.Fatalf("z after re-snapshot = %q, %v, want 9", got, err)
	}
}

// fileSnapshot mirrors the on-disk format Compact writes; the tests craft raw files against this contract.
type fileSnapshot struct {
	ID          string            `json:"id"`
	Term        uint64            `json:"term"`
	CommitIndex uint64            `json:"commit_index"`
	Applied     []string          `json:"applied"`
	Data        map[string]string `json:"data"`
	Suffix      []raft.LogEntry   `json:"suffix"`
}

func writeFileSnapshot(t *testing.T, f fileSnapshot) string {
	t.Helper()
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
