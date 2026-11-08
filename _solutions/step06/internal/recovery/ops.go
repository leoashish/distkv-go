package recovery

import (
	"encoding/json"
	"fmt"
	"os"

	"distkv/internal/raft"
)

// REFERENCE SOLUTION for Step 06. Try it yourself first!

type fileNode struct {
	ID          string          `json:"id"`
	Term        uint64          `json:"term"`
	CommitIndex uint64          `json:"commit_index"`
	Log         []raft.LogEntry `json:"log"`
	Applied     []string        `json:"applied"`
}

func SaveNode(path string, n *raft.Node) error {
	fc := fileNode{ID: n.ID, Term: n.Term, CommitIndex: n.CommitIndex, Log: n.Log, Applied: n.Applied}
	data, err := json.Marshal(fc)
	if err != nil {
		return fmt.Errorf("recovery: encode %s: %w", path, err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("recovery: open temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("recovery: write temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("recovery: sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("recovery: close temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("recovery: rename temp file: %w", err)
	}
	return nil
}

func LoadNode(path string) (*raft.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("recovery: read %s: %w", path, err)
	}
	var fc fileNode
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("recovery: parse %s: %w", path, err)
	}
	if err := verifyNode(fc); err != nil {
		return nil, err
	}
	n := raft.NewNode(fc.ID)
	resp := n.AppendEntries(raft.AppendEntriesRequest{
		Term:         fc.Term,
		LeaderID:     fc.ID,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      fc.Log,
		LeaderCommit: fc.CommitIndex,
	})
	if !resp.Success {
		return nil, fmt.Errorf("recovery: the saved log was rejected on replay (it cannot be consistent)")
	}
	n.CommitIndex = min(fc.CommitIndex, uint64(len(n.Log))) // belt and braces; AppendEntries already advanced it
	n.Applied = fc.Applied                                  // replay reapplied exactly this prefix
	return n, nil
}

func verifyNode(fc fileNode) error {
	for i, e := range fc.Log {
		if e.Index != uint64(i+1) {
			return fmt.Errorf("recovery: %w: entry %d has index %d, want %d", ErrInconsistentLog, i, e.Index, i+1)
		}
		if e.Term > fc.Term {
			return fmt.Errorf("recovery: %w: entry %d has term %d, newer than the node's term %d", ErrInconsistentLog, i, e.Term, fc.Term)
		}
	}
	if fc.CommitIndex > uint64(len(fc.Log)) {
		return fmt.Errorf("recovery: %w: commit index %d exceeds the log length %d", ErrInconsistentLog, fc.CommitIndex, len(fc.Log))
	}
	if uint64(len(fc.Applied)) != fc.CommitIndex {
		return fmt.Errorf("recovery: %w: the saved applied list has %d commands, want exactly the committed prefix (%d)", ErrInconsistentApplied, len(fc.Applied), fc.CommitIndex)
	}
	for i := uint64(0); i < fc.CommitIndex; i++ {
		if fc.Applied[i] != fc.Log[i].Command {
			return fmt.Errorf("recovery: %w: applied[%d] = %q, want the committed entry's command %q", ErrInconsistentApplied, i, fc.Applied[i], fc.Log[i].Command)
		}
	}
	return nil
}
