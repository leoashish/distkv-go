package snapshot

import (
	"distkv/internal/raft"
	"distkv/internal/todo"
)

// STEP 07: implement snapshots. Read docs/steps/step-07-snapshot.md first.   Run:  go test ./internal/snapshot -run Step07

// Compact writes a snapshot of n's applied state to path, ATOMICALLY (write path+".tmp", fsync, os.Rename):
//
//   - the applied KV state (rebuilt by replaying n.Applied through "set key value" interpretation),
//   - the applied prefix itself (so a loader can verify the data rebuilds exactly),
//   - CommitIndex (the snapshot's position in the log) and Term,
//   - the SUFFIX: the entries AFTER CommitIndex (the uncommitted tail, kept honest, never applied).
//
// After Compact returns, the file either contains the whole snapshot or does not exist at all.
func Compact(path string, n *raft.Node) error {
	return todo.NotImplemented("Step 07: Compact")
}

// Load rebuilds a follower from a snapshot: the Store ends holding the snapshot's committed state; the log holds only the
// suffix; CommitIndex, Applied and LastApplied match the snapshot — so the node may receive new entries after the
// snapshot point without re-applying the world.
//
// Verification before anything else (never trust the file):
//   - the suffix is contiguous from CommitIndex+1 and no suffix entry's term exceeds the snapshot's term;
//   - len(Applied) == CommitIndex;
//   - the KV state the replayed Applied commands imply is IDENTICAL to the saved Data map.
//
// Any disagreement → an error, nothing is loaded.
func Load(path string) (*raft.Node, error) {
	return nil, todo.NotImplemented("Step 07: Load")
}
