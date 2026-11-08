# Step 06 — Crash recovery: come back alive

| | |
|---|---|
| **Build** | `internal/recovery/ops.go` — `SaveNode`, `LoadNode` |
| **Given** | `ErrInconsistentLog`, `ErrInconsistentApplied`, `internal/raft.Node`, `internal/raft.NewNode`, `internal/raft.AppendEntries`, `internal/raft.AppendEntriesRequest` (`internal/recovery/recovery.go`) |
| **Tests** | `internal/recovery/recovery_test.go` — 10 tests |
| **Run** | `go test ./internal/recovery -run Step06 -race -v` (step 05 must pass first) |
| **You'll learn** | verified load (never trust the file), replaying through the real AppendEntries path, fsync-then-rename, committed vs uncommitted survival |
| **Time** | ~1.5 h |

## 1. The story

Step 05 gave DistKV a Raft log — but the moment the process dies, everything is gone. A real node restarts and has to know: *which entries were committed?* Those are the only ones the store may replay. Uncommitted entries may be kept in the log (they will never be applied until a new leader commits them) — or lost, which Raft permits.

Step 06 builds crash recovery for a node: **atomically save its world, load it back, verify everything, replay through the real code path**. A file that claims to be "a node" is indistinguishable from random bytes until it passes every check the node itself would apply.

## 2. Concept corner

- **Save = write-tmp + fsync + rename** — the file appears either complete or not at all (POSIX rename is atomic). No half-written nodes on disk.
- **Restart = Follower** — a restarted node is never leader; Raft leaders are re-elected *after* recovery, and they learn the current term from the cluster.
- **Committed is sacred**: entries at index ≤ CommitIndex are applied in order. Entries beyond CommitIndex are kept in the log but **never applied**, and they will never reach the store by accident.
- **Verify on load, then trust nothing**: indices contiguous from 1, no entry term newer than the node's own term, CommitIndex ≤ len(log), Applied == exactly the committed prefix — in order. Any lie → refuse to load.
- **Replay through the same path**: `LoadNode` applies the saved log by calling `AppendEntries` with `LeaderCommit = savedCommitIndex` — the same function a follower runs during normal replication. If the file is weird, the *same* consistency checks the follower uses kill it.
- **You can see this in the tests**: one of them crafts a "free" value that explodes your Applied check. Another crafts a log with an entry from the future.

## 3. Requirements

| Function | Behaviour |
|---|---|
| `SaveNode(path string, n *raft.Node) error` | Marshal `{id, term, commit_index, log, applied}` to JSON, write to `path + ".tmp"`, `fsync`, `Close`, `os.Rename(tmp, path)`. Errors wrapped with `%w`; temp file left behind never happens. |
| `LoadNode(path string) (*raft.Node, error)` | Read (missing → `os.ErrNotExist` wrapped) and parse. **Verify**: contiguous 1-based indices; every entry's term ≤ node term; CommitIndex ≤ log length; Applied is exactly the committed prefix in order. On success, rebuild a fresh follower and replay the saved log through `AppendEntries`. Any mismatch → an error, nothing loaded. |

## 4. What the tests pin down

- Round trip: term, commit index, every log entry (index/term/command), and the applied list come back identically.
- The committed prefix lands in the store without any new proposal (replay does the work).
- Missing file → error wrapping `os.ErrNotExist`; unwritable path → same.
- A log with a gap (index 2 missing) → `ErrInconsistentLog`.
- A commit index beyond the log → `ErrInconsistentLog`.
- An entry whose term is newer than the node's term → `ErrInconsistentLog`.
- An Applied list that lies about the committed prefix → `ErrInconsistentApplied`.
- Uncommitted trailing entries survive in the log, but the store shows only the committed prefix.
- A restarted node is a Follower, may not propose, becomes leader by calling `BecomeLeader`, and serves again.

## 5. Hints (open one at a time)

<details><summary>Hint 1 — the on-disk struct</summary>

`type fileNode struct{ ID string; Term uint64; CommitIndex uint64; Log []raft.LogEntry; Applied []string }` with json tags `id`, `term`, `commit_index`, `log`, `applied`.

</details>

<details><summary>Hint 2 — verification order</summary>

(1) parse; (2) iterate entries: `e.Index == i+1` and `e.Term <= fc.Term`; (3) `fc.CommitIndex <= len(fc.Log)`; (4) `len(fc.Applied) == fc.CommitIndex`; (5) element-wise compare of Applied vs entry commands. Wrap each failure in the right sentinel with `%w`.

</details>

<details><summary>Hint 3 — replay through the REAL path</summary>

After verification, create `raft.NewNode(fc.ID)` and call `n.AppendEntries` with the whole saved log, `PrevLogIndex: 0`, `PrevLogTerm: 0`, `LeaderCommit: fc.CommitIndex`. The node validates and applies the committed prefix exactly as a follower would.

</details>

<details><summary>Hint 4 — no double-apply</summary>

`n.AppendEntries` with `LeaderCommit = fc.CommitIndex` advances the node's commit index and applies via its own `applyLocked` — the same code path as live replication, zero special-casing.

</details>

## 6. Stretch challenges

1. **Snapshot + log suffix:** instead of saving the whole log, snapshot the store every 100 entries and save (snapshot, snapshot-term, snapshot-index, suffix entries). Rebuild = load snapshot + replay suffix.
2. **Metadata double check:** `fsync` the directory after `Rename` — what can't you lose without it?
3. **Corruption fuzzing:** flip every byte of a valid save file, one at a time, and demand that every result is either a parse error, an inconsistency error, or a clean load of identical data.
4. **Partially-committed crash:** craft a file in which every node dies mid-append; what must the *next* leader's recovery replay to keep the system consistent?

## 7. Reflect

- Why does a restarted node come back as a **follower**?
- Why is the *applied* list in the save file checked against the log's committed prefix rather than trusted?
- Why can entries beyond the commit index be kept in the log yet never applied?
- Why does replay use the same code path as live replication? (Hint: tested, shared, and guaranteed identical semantics.)

## 8. Done when

`go test ./internal/recovery -run Step06 -race -v` → **ok** (needs step 05 first: `bash scripts/apply-solution.sh 01 && bash scripts/apply-solution.sh 05 && bash scripts/apply-solution.sh 06`). Then `go vet ./...`, `gofmt -l .`.

## 9. The end — and beyond

Congratulations: you have built a recoverable, replicated, consensus-driven KV store — cell by cell, file by file, hash by hash.

Stretch roadmap if you're still hungry:

- **Step 07 — real networking:** exchange AppendEntries over HTTP between two processes; kill one; watch it rejoin and re-sync.
- **Step 08 — linearizable reads:** leader-lease or ReadIndex reads; measure stale-read rate without them.
- **Step 09 — membership changes:** add/remove a node at runtime (the Raft joint-consensus problem, in miniature).
- **Playbook moment:** point a RESP-like parser at this node and serve real traffic.
