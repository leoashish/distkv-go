# Step 02 — The write-ahead log: durability and crash recovery

| | |
|---|---|
| **Build** | `internal/wal/record.go` (`EncodeRecord`, `DecodeRecord`, `ScanRecords`); `internal/wal/log.go` (`Open`, `Append`, `AppendBatch`, `Sync`, `Close`, `Len`, `Recovered`, `ReadAll`) |
| **Given** | `HeaderSize`, `MaxRecordSize`, `ErrTruncated`, `ErrCorrupt`, `ErrClosed`, `ErrTooLarge`, `Options`, `WAL` (`internal/wal/wal.go`) |
| **Tests** | `internal/wal/wal_test.go` — 16 tests |
| **Run** | `go test ./internal/wal -run Step02 -race -v` |
| **You'll learn** | WAL framing, CRC-32C checksums, torn writes, group commit, fsync poisoning |
| **Time** | ~2–3 h |

## 1. The story

Step 01 gave DistKV a brain: an in-memory map. Correct, fast, and **gone the moment the process dies**.

The next promise of a real store: *"an acknowledged write survives a crash."* The mechanism underneath that promise is the **write-ahead log (WAL)** — the single most important structure in data storage (Postgres WAL, SQLite journal, RocksDB, Kafka, etcd, every LSM-tree of Chapter 20).

The idea:

1. **Before mutating in-memory state**, append the intent to a log file.
2. **fsync** (make it durable).
3. **Ack** the caller; do the slow in-place mutation afterwards (or just keep the log as source of truth and replay later).
4. On startup, **replay** the log to rebuild state.

A log is chosen over in-place writes because appends are sequential and cheap, whereas in-place updates touch random pages and can be left half-done by a crash.

## 2. Concept corner

- **"A write returned" does not mean "the data is on disk".** `f.Write(b)` copies into the **OS page cache** and returns; the OS flushes to disk (which has its own cache) later. The durability call is **`fsync`** (`f.Sync()`). Expensive (ms), which shapes everything below.
- **Why a log, and why it is fast:** a database stores data in complicated structures (B-trees, hash files) updated **in place** — random writes, several pages per update, half-updated state after a crash. A WAL instead **appends** one small record per change: sequential, one `fsync`, no corruption risk (we only ever add at the end).
- **Framing:** a log is a byte stream, so each record is framed: 4 bytes length (LE) + 4 bytes CRC-32C (LE, computed over the length bytes **followed by** the payload) + payload. The checksum covers the length too, because a corrupt length would shift where the next record starts.
- **Torn write vs corruption:** a crash mid-append leaves a **truncated** tail (normal → silently repaired). A complete record with a bad checksum is **corrupt** (bit rot / bad disk / partial overwrite → the media can't be trusted). A length above `MaxRecordSize` is **corrupt**, never merely truncated — never try to read or allocate that much.

## 3. Requirements

Implement:

| Piece | Behaviour |
|---|---|
| `EncodeRecord(payload) []byte` | Frame one payload: LE length + LE CRC-32C(length‖payload) + payload. Empty payload is valid. |
| `DecodeRecord(b) (payload, n, err)` | Parse one record from `b` with errors checked **in order**: incomplete header → `ErrTruncated`; length > `MaxRecordSize` → error wrapping `ErrCorrupt`; incomplete body → `ErrTruncated`; checksum mismatch → error wrapping `ErrCorrupt`. Payload must be **copied**. |
| `ScanRecords(b) (records, validBytes, err)` | Decode back-to-back until bytes run out or a record fails; return records, the byte length of the valid prefix, and why scanning stopped. |
| `Open(path, opts)` | Read file (missing = OK, start empty); scan; if there are bytes after the valid prefix, **truncate** the file to `validBytes`; open for append. Returns `Len() = number of recovered records`, `Recovered() = (records, bytesCut)`. |
| `Append(payload) (seq, err)` | Append one record, seq starts at 1 and is contiguous. With `opts.SyncEveryAppend`, fsync before returning (durability!). Oversize → `ErrTooLarge`, write nothing. |
| `AppendBatch(payloads) (firstSeq, err)` | Write many records with **one write call** and (if asked) **one fsync** = group commit; returns seq of the first. Rejected batch writes nothing. |
| `Sync() error` | fsync now. |
| `Close() error` | fsync once, close; idempotent; use after close → `ErrClosed`. |
| `Len() uint64` | Number of valid records. |
| `Recovered() (int, int64)` | Records recovered at Open + bytes truncated off the tail. |
| `ReadAll(path) ([][]byte, error)` | Read all valid records of a file (missing file → error). |

Cross-cutting: after a **failed fsync the log is POISONED** (never retry — the kernel may have already dropped the data; the classic fsync-gate bug); use a mutex; everything safe for concurrent use.

## 4. What the tests pin down

- Encoding matches the documented layout exactly.
- Round trip + payload is a copy; empty payload OK.
- Every prefix of a record is `ErrTruncated`; any single-bit flip is detected (corrupt/truncated).
- A 4 GiB length field is `ErrCorrupt`, not truncated.
- `ScanRecords` reports the valid prefix and why it stopped (clean / torn / corrupt).
- Appends numbered from 1, survive reopening; `ReadAll` reads them back; missing file is an error.
- Torn tail → recovery truncates the file, appends cleanly afterwards.
- Mid-log corruption → drop damaged record + everything after.
- **Crash at every byte offset** of a log → exactly the complete records survive; new append after recovery is readable.
- Group commit: one fsync for a 100-record batch; without `SyncEveryAppend`, fsync only on demand; close syncs once.
- Failed fsync poisons the log (no retry, error propagates).
- Oversize batch rejected atomically; closed-log behaviour.

## 5. Hints (open one at a time)

<details><summary>Hint 1 — CRC over two pieces</summary>

`table := crc32.MakeTable(crc32.Castagnoli)`; `crc := crc32.Update(0, table, lengthBytes)`; `crc = crc32.Update(crc, table, payload)`. Write it with `binary.LittleEndian.PutUint32`.

</details>

<details><summary>Hint 2 — DecodeRecord must copy</summary>

Return `append([]byte(nil), payload...)`: callers reuse their read buffer, and a record that aliases it changes under their feet.

</details>

<details><summary>Hint 3 — Open</summary>

`data, err := os.ReadFile(path)` (missing file is fine); `records, valid, _ := ScanRecords(data)`; if `valid < len(data)` → `os.Truncate(path, valid)`; then `os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)`.

</details>

<details><summary>Hint 4 — poisoning</summary>

A `poisoned error` field. `syncLocked`: if poisoned return it; call `SyncFunc`; on error store it *and* return it. Check poisoned at the top of `AppendBatch` too.

</details>

## 6. Stretch challenges

1. **Segment the log:** roll to a new file at 64 MB; recovery scans only the newest segment.
2. **Checkpoint + replay:** snapshot a state machine every N records; recover = load snapshot + replay tail.
3. **fsync the directory** after creating the log file (and after rolling segments). What can be lost without it?
4. **Background group commit:** collect concurrent appends for up to 1 ms, write as one batch with one fsync, release all callers. Measure.
5. **CRC chaining:** each record's CRC covers the previous record's CRC. What does that catch that per-record CRCs cannot?

## 7. Reflect

- What exactly does `f.Write` guarantee, and what does `f.Sync` add?
- Why does recovery *truncate* rather than ignore the bad tail?
- Why is a 4 GiB length "corrupt" rather than "truncated"?
- Why must a failed `fsync` poison the log instead of being retried?

## 8. Done when

`go test ./internal/wal -run Step02 -race -v` → **ok**. Then `go vet ./...`, `gofmt -l .`, and compare with `_solutions/step02/…`.

## 9. Next

Step 03 — **Replication**: leader + followers, quorum writes and reads. The WAL you just built is what the leader will write and what followers will replay.
