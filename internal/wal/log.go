package wal

import (
	"distkv/internal/todo"
)

// Open reads the whole file if it exists (a missing file is fine: start empty), ScanRecords it, and if there are bytes
// after validBytes (a torn tail, or corruption) TRUNCATES THE FILE to validBytes (os.Truncate) so new appends start on a
// clean boundary -- appending after garbage would hide every later record behind it. Then opens the file for appending
// (os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644). Len() counts the valid records; Recovered() returns (records found at
// Open, bytes cut off the tail).
func Open(path string, opts Options) (*WAL, error) {
	return nil, todo.NotImplemented("Step 02: Open")
}

// Append writes ONE record; seq numbers are 1-based and contiguous: the first record ever is 1.
// AppendBatch writes several with ONE write call and (if SyncEveryAppend) ONE fsync -- "group commit". It returns the
// seq of the FIRST record. A payload over MaxRecordSize -> ErrTooLarge and NOTHING is written (check all before writing
// any).
func (w *WAL) Append(payload []byte) (uint64, error) {
	return 0, todo.NotImplemented("Step 02: Append")
}

func (w *WAL) AppendBatch(payloads [][]byte) (uint64, error) {
	return 0, todo.NotImplemented("Step 02: AppendBatch")
}

// Sync calls the sync function. Close() syncs once, closes the file, and is idempotent; every use after Close returns
// ErrClosed.
func (w *WAL) Sync() error { return todo.NotImplemented("Step 02: Sync") }

func (w *WAL) Close() error { return todo.NotImplemented("Step 02: Close") }

// Len counts the valid records.
func (w *WAL) Len() uint64 { return 0 }

// Recovered returns (records found at Open, bytes cut off the tail).
func (w *WAL) Recovered() (int, int64) { return 0, 0 }

// ReadAll returns every VALID record of a log file without opening it for writing (it stops at a torn or corrupt
// tail silently). A missing file is an error (os.ReadFile's).
func ReadAll(path string) ([][]byte, error) {
	return nil, todo.NotImplemented("Step 02: ReadAll")
}
