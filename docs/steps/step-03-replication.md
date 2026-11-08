# Step 03 — Replication: leader, followers, quorum writes

| | |
|---|---|
| **Build** | `internal/replica/ops.go` — `NewCluster`, `Cluster.{Put, Get, FollowerGet, Crash, Heal}`, `Replica.{Get, Set, Delete}` |
| **Given** | `Replica`, `Cluster`, `Role`, sentinels `ErrNoQuorum`, `ErrReplicaDown`, `ErrNoFollower` (`internal/replica/replica.go`); `internal/kv.Store` (step 01) |
| **Tests** | `internal/replica/replica_test.go` — 11 tests |
| **Run** | `go test ./internal/replica -run Step03 -race -v` |
| **You'll learn** | leader/follower replication, quorum writes, rollback on failed commit, the limits of "synchronous" replication without consensus |
| **Time** | ~2 h |

## 1. The story

Step 02 made DistKV durable on ONE node. A distributed store must survive losing nodes entirely.

Step 03 builds the oldest replication design in the book: **one leader accepts writes, it forwards every write to every follower, and the write "commits" only when a majority acknowledges it.** No consensus yet — this step is about *what quorum buys you*, and *what it costs*.

## 2. Concept corner

- **Quorum = majority of the cluster**: `total / 2 + 1`. For 3 nodes, 2; for 5, 3. Any two majorities **intersect** in at least one node — that intersection is why the cluster cannot forget an acknowledged write.
- **Synchronous replication**: the leader waits for follower ACKs before acknowledging the client. Simple, safe, and slow — the price of consistency.
- **Rollback on quorum failure**: if the write does not get a majority, every replica that saw the write is reverted to the pre-write state. A rejected write is *invisible* — no partial truths.
- **Crash and Heal**: a crashed follower refuses reads and writes; when it heals it has *missed every write made in between* — replication without consensus delivers **permanent divergence**. This is not a bug in our code; it is the boundary consensus (steps 04–05) exists to close.

## 3. Requirements

| Function | Behaviour |
|---|---|
| `NewCluster(names ...string) *Cluster` | `names[0]` is the leader; the rest are followers; majority = `len(names)/2 + 1`. Zero names → a single default node (never nil). |
| `Cluster.Put(ctx, key, value) error` | Leader writes first; every follower is asked; count ACKs (leader included). Majority → commit. Otherwise **roll back** every replica that got the write to the pre-write value (or delete the key) → `ErrNoQuorum`. |
| `Cluster.Get(ctx, key) (string, error)` | Leader read. Missing key → error wrapping `kv.ErrNotFound`. |
| `Cluster.FollowerGet(ctx, i, key) (string, error)` | Read from follower `i`. Down → `ErrReplicaDown`. `i` out of range → `ErrNoFollower`. |
| `Replica.Get/Set/Delete` | The per-replica CRUD over `kv.Store`, refused when crashed (`ErrReplicaDown`). |
| `Cluster.Crash(i) / Heal(i)` | Toggle follower `i`'s health. Out-of-range indices are ignored. |

## 4. What the tests pin down

- Leadership is assigned to `names[0]`; roles are set; followers are replicas.
- Read-after-write through the leader.
- Every healthy follower receives the write.
- Quorum (leader + 1 follower of 3) commits even with one follower down.
- Majority lost (both followers down) → `ErrNoQuorum`, and the **leader reverts** (nothing heard it).
- Rolling back an already-existing key restores the ORIGINAL value, not a wipe.
- A crashed follower misses the write; even after Heal it serves the old value forever (divergence — documented, not fixed; steps 04–05 fix it).
- Consecutive writes overwrite correctly.
- Missing key errors wrap `kv.ErrNotFound`; out-of-range follower index → `ErrNoFollower`; crash/heal round-trips resume accepting writes.

## 5. Hints (open one at a time)

<details><summary>Hint 1 — the cluster is just a big struct</summary>

`leader`, `followers []*Replica`, `majority int`. `NewCluster` fills them.

</details>

<details><summary>Hint 2 — read the old value first</summary>

Before writing anything, read the leader's current value (`getRaw`). You need it for rollback.

</details>

<details><summary>Hint 3 — ack counting</summary>

`acks := 1` (leader) + every follower whose `Set` succeeds; `touched` collects the replicas you must undo if the count fails.

</details>

<details><summary>Hint 4 — rollback</summary>

If `acks < majority`: for each replica in `touched`, `Set(ctx, key, old)` if the key existed, else `Delete(ctx, key)`. Return `ErrNoQuorum`.

</details>

## 6. Stretch challenges

1. **Async replication:** give the follower a delay; show the window where the leader acknowledges before the follower persists. Then fix it (quorum again).
2. **Lease reads:** a leader lease (time-based) makes leader reads safe without a quorum round-trip. Simulate: only the leaseholder leader may serve reads.
3. **Fix the divergence:** after Heal, have the follower request every record it missed from the leader (a mini catch-up protocol). What can go wrong (hint: you need versions/indices — step 04).
4. **Promotion on crash:** refuse writes while the leader may be partitioned; how does the cluster promote a follower? (That's step 04's election.)

## 7. Reflect

- What does quorum give you that "write to everyone" does not? (Availability!)
- What does quorum NOT give you? (A single agreed order of operations — see steps 04–05.)
- Why is rollback necessary — what if you left the half-written state?
- Why does Heal not fix the diverged follower's data? What would fix it?

## 8. Done when

`go test ./internal/replica -run Step03 -race -v` → **ok**. Then `go vet ./...`, `gofmt -l .`, and compare with `_solutions/step03/…` (apply step01 first: `bash scripts/apply-solution.sh 01 && bash scripts/apply-solution.sh 03`).

## 9. Next

Step 04 — **leader election**: terms, votes and heartbeats, so a stale leader cannot pretend forever.
