# Step 07 — Snapshots: compact the applied state, keep the suffix

| | |
|---|---|
| **Build** | `internal/snapshot/ops.go` — `Compact`, `Load` |
| **Given** | `ErrInvalidSuffix`, `ErrInvalidSnapshot` (`internal/snapshot/snapshot.go`); `internal/raft.Node` — evolved here with **`SnapBase`/`SnapTerm`** and an exported **`LastApplied`** cursor (`internal/raft/raft.go`) |
| **Tests** | `internal/snapshot/snapshot_test.go` — 10 tests |
| **Run** | `go test ./internal/snapshot -run Step07 -race -v` (steps 01–06 must pass first) |
| **You'll learn** | snapshotting, suffix retention, verify-don't-trust-on-load, the replay boundary, why the node must know its own snapshot point |
| **Time** | ~2–3 h |

## 1. The story

Step 06 saved the node's *world* — the whole log — atomically and replayed it on restart. Fine while logs are small.

Production logs grow forever; replaying every entry ever is not viable. Step 07 introduces the mechanism every real system uses instead of "replay everything": a **snapshot**.

A snapshot is the applied state as of some commit index, plus the **suffix** — the uncommitted entries after it. Recovery = load the snapshot (state) + replay the suffix (only the new stuff). Steps: discard everything before `CommitIndex`, keep it, and prove afterwards you can rebuild the truth from it.

## 2. Concept corner

- **Snapshot = the state machine's photo at CommitIndex**: the KV map of "set key value" commands from the committed prefix, rebuilt by re-executing `Applied`. Two independent witnesses must agree — the data map and the applied list — or the file is a lie.
- **The suffix survives, uncommitted**: entries *after* CommitIndex are kept in the snapshot but are **never applied**. They will ride along until a future leader commits them (or forever if they never get committed — Raft never promises).
- **The node must know its own snapshot boundary**: `SnapBase`/`SnapTerm` — entries ≤ SnapBase are gone from `Log` and never re-applied; the snapshot is the *only* match position for anything at or below it. This is why non-obvious API members (`LastApplied`, `SnapTerm`) were added to the `Node` in Step 05 — real systems evolve their types exactly like this.
- **Verify, then trust**: a snapshot that lies about its own commands cannot produce the same map — we re-execute `Applied` and compare maps element by element. A file matching on every byte is still not allowed to boot.
- **Indices stay ABSOLUTE**: even with a truncated prefix, every entry keeps its absolute index — `CommitIndex`, `Applied`, `LastApplied`, `Index` — one consistent coordinate system, snapshots or not.

## 3. Requirements

| Function | Behaviour |
|---|---|
| `Compact(path string, n *raft.Node) error` | Build `{id, term, commit_index, base_term, applied, data, suffix}`; `data` is rebuilt by re-executing `n.Applied`; `suffix` keeps **uncommitted** entries (absolute indices > CommitIndex); write atomic (temp+fsync+rename). |
| `Load(path string) (*raft.Node, error)` | Read and **fully verify**: suffix contiguous from CommitIndex+1; suffix terms ≤ snapshot term; `len(Applied) == CommitIndex`; `Applied` re-executed == `Data`. Then rebuild: store = Data, suffix = Log, CommitIndex, Applied, **LastApplied = CommitIndex**, **SnapBase = CommitIndex**, **SnapTerm = BaseTerm**. Role = Follower. |

## 4. What the tests pin down

- Round trip: term, commit index, applied list — byte-identical.
- The loaded store holds the entire committed state (multi-key, overwritten value wins).
- An uncommitted suffix survives the load but never lands in the store; commit index and applied list untouched.
- Atomic write — no temp file left behind.
- Missing file → error wrapping `os.ErrNotExist`.
- A suffix with a gap → `ErrInvalidSuffix`; a suffix entry from the future → `ErrInvalidSuffix`.
- The applied list of the wrong length → `ErrInvalidSnapshot`.
- Data that contradicts the applied commands → `ErrInvalidSnapshot`.
- A reloaded node accepts new proposals (after `BecomeLeader`), keeps its prior writes, and survives a second snapshot round-trip.

## 5. Hints (open one at a time)

<details><summary>Hint 1 — building the snapshot</summary>

`fc := {id, term, commitIndex, applied (copy), data: replay(applied), suffix: entries with Index > commitIndex}`. If `CommitIndex > 0`, find the entry **at absolute index CommitIndex** — its term is `base_term` (before any deletion, that is `Log[CommitIndex-1]`).

</details>

<details><summary>Hint 2 — verifying an unknown file</summary>

(1) suffix contiguity: `suffix[i].Index == CommitIndex+i+1`; (2) suffix terms ≤ snapshot term; (3) `len(Applied) == CommitIndex`; (4) re-execute `Applied` and compare against `Data` (`len` + element-wise).

</details>

<details><summary>Hint 3 — rebuilding the node</summary>

`n := raft.NewNode(id); n.Term = …; n.Role = Follower`; replay `Data` into `n.Store`; set `n.Log = suffix`, `n.CommitIndex`, `n.Applied`, and **critically** `n.LastApplied = fc.CommitIndex` and `n.SnapBase/n.SnapTerm`.

</details>

<details><summary>Hint 4 — the absolute index discipline</summary>

Everything in the file uses absolute indices; the truncated prefix is gone forever, the suffix keeps its absolute numbers. Never write `n.Log[CommitIndex:]` — filter by `e.Index > fc.CommitIndex`.

</details>

## 6. Stretch challenges

1. **Chain the snapshots:** snapshot every 1000 entries automatically; reload = latest snapshot + suffix replay.
2. **Checksum the applied state:** add a SHA-256 of the serialized `data` map; verify on load before doing the two-way comparison.
3. **Compact the WAL itself (step 02):** truncate all entries with index ≤ snapshot point. How does the recovery scan stay correct?
4. **Re-verify without trusting:** prove (by hand, on paper) why the replayed `Applied` and `Data` agreeing implies the snapshot was honest.

## 7. Reflect

- Why does the snapshot keep the uncommitted suffix at all?
- Why must `LastApplied` be set to CommitIndex on load — what breaks if it stays 0?
- What does the follower's `SnapBase` guarantee about anything before it — and why can the loader never trust `Log` alone?
- Why is the re-execution of `Applied` the only honest way to build `data`? 

## 8. Done when

`go test ./internal/snapshot -run Step07 -race -v` → **ok** (needs steps 01–06 first: `for s in 01 02 03 04 05 06; do bash scripts/apply-solution.sh $s; done && bash scripts/apply-solution.sh 07`). Then `go vet ./...`, `gofmt -l .`.

## 9. The end — and beyond

DistKV now stores durable state, replicates it, elects leaders, survives crashes, and compacts itself. What it doesn't have yet:

- **Step 08 — real networking:** AppendEntries over HTTP between two processes; kill one; watch it rejoin.
- **Step 09 — snapshot install for lagging followers:** a leader that knows a follower's `SnapBase` ahead of time ships the snapshot instead of the log.
- **Step 10 — membership change:** add a node at runtime — the joint-consensus problem in miniature.
