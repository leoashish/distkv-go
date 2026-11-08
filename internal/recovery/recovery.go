// Package recovery rebuilds a Raft node from disk after a crash — and trusts nothing until it has re-verified it.
package recovery

import "errors"

var (
	// ErrInconsistentLog: the saved log's indices are not contiguous from 1, an entry's term is newer than the node's own
	// term, or CommitIndex exceeds the log length. GIVEN.
	ErrInconsistentLog = errors.New("recovery: the saved log is inconsistent")
	// ErrInconsistentApplied: the saved Applied list is not exactly the commands of the committed prefix, in order. GIVEN.
	ErrInconsistentApplied = errors.New("recovery: the saved applied list is not the committed prefix")
)
