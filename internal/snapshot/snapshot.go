// Package snapshot compacts a Raft node's applied state into a file, so recovery never has to replay an endless log.
package snapshot

import "errors"

var (
	// ErrInvalidSuffix: the saved suffix is not contiguous from CommitIndex+1, or an entry is newer than the snapshot's term. GIVEN.
	ErrInvalidSuffix = errors.New("snapshot: the saved suffix is inconsistent")
	// ErrInvalidSnapshot: the committed state and the applied list disagree (the snapshot's data does not match what its own log applied). GIVEN.
	ErrInvalidSnapshot = errors.New("snapshot: the committed state and the applied list disagree")
)
