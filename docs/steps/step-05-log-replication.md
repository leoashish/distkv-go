# Step 05 — Raft: log replication and the commit index

| | |
|---|---|
| **Build** | `internal/raft/ops.go` — `NewNode`, `Node.{BecomeLeader, AppendEntries, Propose}` |
| **Given** | `Role`, sentinels `ErrNotLeader`, `ErrNoQuorum`, `LogEntry`, `AppendEntriesRequest/Response`, `Node` (`internal/raft/raft.go`); `internal/kv.Store` (step 01) |
| **Tests** | `internal/raft/raft_test.go` — 10 tests |
| **Run** | `go test ./internal/raft -run Step05 -race -v` |
| **You'll learn** | replicated logs, the commit index, apply-before-notify, diverged log repair by truncation, one-entry-one-slot consistency |
| **Time** | ~3 h |

## 1. The story

Steps 03–04 gave DistKV replication and elections — but no way to agree on *what* was written, in what order.

Step 05 builds the heart of Raft in miniature: **a replicated log**. The leader appends commands to its log, ships them to the followers, and the moment a majority holds an entry, that entry is **committed** — and every node applies it to its state machine. Once committed, it is permanent (Raft's Leader Completeness Property builds on it).

## 2. Concept corner

- **The log is the truth**: every committed command is in at least one majority — two majorities intersect, so the truth cannot be lost by any subset of nodes dying.
- **Index + term**: every entry carries the leader's term at creation time and a contiguous 1-based index. Term+Index is the unique identity of an entry.
- **PrevLogIndex/PrevLogTerm**: consistency check for the follower — "your log must match mine at this point"; otherwise refuse.
- **Conflict repair by truncation**: if the follower's entry at the same index differs in term, it is deleted (and everything after it), and the leader's tail is appended. This repairs a diverged follower.
- **Commit index**: entries become visible the moment a **majority** holds them. `CommitIndex = min(LeaderCommit, len(log))` on the follower — then apply everything up to it.
- **Two phases**: (1) replicate + majority ACK → leader sets CommitIndex, applies; (2) heartbeat round → followers learn the new commit index, apply too.
- **Applied in order**: `set key value` strings are applied to the KV store in log order — deterministic, identical on every node.

## 3. Requirements

| Function | Behaviour |
|---|---|
| `NewNode(id string) *Node` | Follower at term 0; an empty log and an empty KV store. |
| `Node.BecomeLeader()` | Term +1; `Role = Leader`. |
| `Node.AppendEntries(req AppendEntriesRequest) AppendEntriesResponse` | Reject stale terms; check `PrevLogIndex/PrevLogTerm` matches the follower's log (else refuse); for each entry, truncate the follower's log from the first mismatched position and append the incoming tail; advance `CommitIndex = min(LeaderCommit, len(log))`; apply every newly committed entry (`set k v` → KV store) and record it in `Applied`. |
| `Node.Propose(command string, others []*Node) error` | Leader only (`ErrNotLeader`). Append the entry; send `AppendEntries` (PrevLogIndex = own len−1 or 0) to every node in `others`; tally ACKs (leader included); on majority → commit (index + apply) + heartbeat round so followers commit too; otherwise `ErrNoQuorum` (entry stays in the log, **uncommitted**). |

## 4. What the tests pin down

- New nodes start as followers at term 0 with an empty log and a working store.
- `BecomeLeader` opens a new term.
- Solo cluster: propose → committed and applied; `CommitIndex = 1`; Applied list and store reflect it.
- 3-node: propose = replicated + committed everywhere; followers apply after the heartbeat round.
- A follower that receives an entry but no commit notification applies NOTHING (commit index 0, store unchanged); after the heartbeat with `LeaderCommit = 1`, it applies.
- A diverged follower with a stale-term bogus entry gets it **truncated and replaced** with the leader's entry (a newer term always wins).
- Without a majority (only f1 grants out of 5): `ErrNoQuorum`, CommitIndex stays 0, the leader's entry survives, nothing is applied.
- Proposing from a follower → `ErrNotLeader`.
- Commands apply in propose order; later writes win.
- Indices are one-based and contiguous.

## 5. Hints (open one at a time)

<details><summary>Hint 1 — the consistency check</summary>

If `req.PrevLogIndex > 0`: require `len(n.Log) >= req.PrevLogIndex` and `n.Log[req.PrevLogIndex-1].Term == req.PrevLogTerm` — else return `{Term: n.Term}` with Success=false.

</details>

<details><summary>Hint 2 — the truncation loop</summary>

For each `e` in `req.Entries`: `idx := e.Index - 1`; if `len(n.Log) > idx && n.Log[idx].Term != e.Term` → `n.Log = n.Log[:idx]`; then if `len(n.Log) == idx` → append `e`.

</details>

<details><summary>Hint 3 — commit advance</summary>

`if req.LeaderCommit > n.CommitIndex { n.CommitIndex = min(req.LeaderCommit, uint64(len(n.Log))); n.applyLocked() }` where `applyLocked` replays from `lastApplied+1` to CommitIndex, parsing `set k v` with `strings.Fields`.

</details>

<details><summary>Hint 4 — Propose in two phases</summary>

(1) Append entry, capture `PrevLogTerm` from your own log, send to all `others`; if `< majority(others+1)` acks → `ErrNoQuorum` (do NOT commit). (2) Set `CommitIndex = entry.Index`, apply; then send a second heartbeat round with `LeaderCommit = n.CommitIndex` so followers commit too.

</details>

## 6. Stretch challenges

1. **Snapshots:** after applying 100 entries, snapshot the applied state and drop the prefix; reload = snapshot + replay the tail.
2. **Read-your-writes from followers:** use the commit index as a freshness token — a follower that has applied your write index may serve you.
3. **Randomised election timeouts** (step 04): add `time.AfterFunc`-driven elections and let the cluster self-heal after you kill the leader mid-test.
4. **Fuzz:** generate random propose/truncate sequences and compare the log against a simple oracle model.

## 7. Reflect

- Why do entries carry terms, and what does a mismatch tell the follower?
- Why is the 165-line gap between "leader committed" and "follower committed" closed by a heartbeat-only AppendEntries?
- Why can an entry be in a follower's log but NOT committed? (And why must Step 06's recovery not resurrect it?)
- Why does Raft commit in order but not necessarily *learn* of commits in order?

## 8. Done when

`go test ./internal/raft -run Step05 -race -v` → **ok** (apply step 01 first: `bash scripts/apply-solution.sh 01 && bash scripts/apply-solution.sh 05`). Then `go vet ./...`, `gofmt -l .`.

## 9. The end — and beyond

Congratulations: you have built, in miniature, the technology beneath etcd, Consul, and every Kubernetes control plane.

Stretch roadmap if you're still hungry:

- **Step 06 — recovery:** persist WAL (step 02!) + snapshot + restart; re-elect; verify no committed entry is lost.
- **Step 07 — real networking:** serve AppendEntries over HTTP between two processes on localhost; kill one and watch it rejoin.
- **Step 08 — linearizable reads:** implement ReadIndex/lease reads; measure stale-read rates without them.
- **Playbook moment:** wire `distkv-go` step 05's Raft under `internal/kv.Store` and run a RESP-like protocol against it.
