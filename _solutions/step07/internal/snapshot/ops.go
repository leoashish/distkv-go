package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"distkv/internal/raft"
)

// REFERENCE SOLUTION for Step 07. Try it yourself first!

type fileSnapshot struct {
	ID          string            `json:"id"`
	Term        uint64            `json:"term"`
	CommitIndex uint64            `json:"commit_index"`
	BaseTerm    uint64            `json:"base_term"`
	Applied     []string          `json:"applied"`
	Data        map[string]string `json:"data"`
	Suffix      []raft.LogEntry   `json:"suffix"`
}

func Compact(path string, n *raft.Node) error {
	fc := fileSnapshot{
		ID:          n.ID,
		Term:        n.Term,
		CommitIndex: n.CommitIndex,
		Applied:     append([]string(nil), n.Applied...),
		Data:        applyAll(n.Applied),
	}
	for _, e := range n.Log {
		if e.Index > fc.CommitIndex {
			fc.Suffix = append(fc.Suffix, e)
		}
	}
	if fc.CommitIndex > 0 {
		fc.BaseTerm = baseTerm(n, fc.CommitIndex)
	}
	data, err := json.Marshal(fc)
	if err != nil {
		return fmt.Errorf("snapshot: encode %s: %w", path, err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("snapshot: open temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("snapshot: write temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("snapshot: sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("snapshot: close temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("snapshot: rename temp file: %w", err)
	}
	return nil
}

// baseTerm returns the term of the entry at the given absolute index: the entry is found in the log if present,
// otherwise — when at is exactly the snapshot base — the snapshot's own recorded term of that entry.
func baseTerm(n *raft.Node, at uint64) uint64 {
	for _, e := range n.Log {
		if e.Index == at {
			return e.Term
		}
	}
	if at == n.SnapBase {
		return n.SnapTerm
	}
	return 0
}

func applyAll(cmds []string) map[string]string {
	out := map[string]string{}
	for _, cmd := range cmds {
		parts := strings.Fields(cmd)
		if len(parts) == 3 && parts[0] == "set" {
			out[parts[1]] = parts[2]
		}
	}
	return out
}

func Load(path string) (*raft.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot: read %s: %w", path, err)
	}
	var fc fileSnapshot
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("snapshot: parse %s: %w", path, err)
	}
	if err := verify(fc); err != nil {
		return nil, err
	}
	n := raft.NewNode(fc.ID)
	n.Term = fc.Term
	n.Role = raft.Follower
	for k, v := range fc.Data {
		if err := n.Store.Put(context.Background(), k, v); err != nil {
			return nil, fmt.Errorf("snapshot: restore %q: %w", k, err)
		}
	}
	n.Log = fc.Suffix
	n.CommitIndex = fc.CommitIndex
	n.Applied = append([]string(nil), fc.Applied...)
	n.LastApplied = fc.CommitIndex
	n.SnapBase = fc.CommitIndex
	n.SnapTerm = fc.BaseTerm
	return n, nil
}

func verify(fc fileSnapshot) error {
	for i, e := range fc.Suffix {
		want := fc.CommitIndex + uint64(i) + 1
		if e.Index != want {
			return fmt.Errorf("snapshot: %w: suffix entry %d has index %d, want %d", ErrInvalidSuffix, i, e.Index, want)
		}
		if e.Term > fc.Term {
			return fmt.Errorf("snapshot: %w: suffix entry %d has term %d, newer than the snapshot's term %d", ErrInvalidSuffix, e.Index, e.Term, fc.Term)
		}
	}
	if uint64(len(fc.Applied)) != fc.CommitIndex {
		return fmt.Errorf("snapshot: %w: the applied list has %d commands, want exactly the committed prefix (%d)", ErrInvalidSnapshot, len(fc.Applied), fc.CommitIndex)
	}
	want := applyAll(fc.Applied)
	if len(want) != len(fc.Data) {
		return fmt.Errorf("snapshot: %w: applied commands imply %d keys, the saved data has %d", ErrInvalidSnapshot, len(want), len(fc.Data))
	}
	for k, v := range want {
		got, ok := fc.Data[k]
		if !ok || got != v {
			return fmt.Errorf("snapshot: %w: applied implies %q = %q, the saved data says %q", ErrInvalidSnapshot, k, v, got)
		}
	}
	return nil
}
