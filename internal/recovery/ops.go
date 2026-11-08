package recovery

import (
	"distkv/internal/raft"
	"distkv/internal/todo"
)

// STEP 06: implement crash recovery. Read docs/steps/step-06-recovery.md first.   Run:  go test ./internal/recovery -run Step06

// SaveNode writes n's replicated state to path ATOMICALLY: write to path+".tmp", fsync, then os.Rename. It saves the
// node's Term, the full log, CommitIndex, and the Applied command list. After SaveNode returns, the file either contains
// the complete state or does not exist at all.
func SaveNode(path string, n *raft.Node) error {
	return todo.NotImplemented("Step 06: SaveNode")
}

// LoadNode reads a saved node file and, BEFORE returning it, fully verifies it:
//   - the log's indices are contiguous from 1 and no entry's term exceeds the node's term;
//   - CommitIndex does not exceed the log length;
//   - Applied is exactly the commands of the first CommitIndex entries, in order.
//
// On success it rebuilds a *raft.Node (Role=Follower, as any restarted node is) and replays the log through the real
// AppendEntries path, so the store ends up holding exactly the committed prefix — the same state a leader would see.
// A tampered file never loads.
func LoadNode(path string) (*raft.Node, error) {
	return nil, todo.NotImplemented("Step 06: LoadNode")
}
