# DistKV (Go) — learn distributed systems by building a distributed key-value store, step by step

A hands-on course in **distributed systems**, built the same way as [shopflow-go](../shopflow-go) and
[dataflow-go](../dataflow-go): **one step at a time, each with a PRD, failing tests, stubs, collapsible hints,
stretch challenges and a reference solution**.

Today you are building **DistKV** — a small distributed key-value store — out of first principles, in Go. Step 01
gives a node its brain (an in-memory map). Step 02 gives it a **write-ahead log**. Step 03 gives it **replication**
with quorum writes. Step 04 gives it **leader elections**. Step 05 gives it **Raft log replication and a commit
index** — the machinery under etcd, Consul and every Kubernetes control plane. Step 06 makes a node survive a
full process restart, rebuilt and verified from disk. Step 07 teaches it to **snapshot** — compact the applied
state, keep the suffix, never replay the universe.

The goal: every day you add one small, testable piece of distribution, and you *understand* why it is there —
because you wrote it, tested it red-first, and debugged it.

## What you are building, and why this shape

A distributed key-value store is the smallest possible "real" distributed system, and it contains every lesson in order:

1. **Concurrency** — a single node must be safe to use from many goroutines. (Step 01.)
2. **Durability** — what happens when the process dies? Write-ahead log. (Step 02.)
3. **Consistency** — who owns a write, and how many copies must agree? Replication + quorum. (Step 03.)
4. **Fault tolerance** — what if the owner dies? Elections. (Step 04.)
5. **Consensus** — how do followers learn the truth? Log replication + commit index. (Step 05.)

By the end you have something every distributed database has in common, at a size that fits in a README —
**so you know where the textbooks get it right and where they make trade-offs you haven't felt yet.**

## Start here

```bash
cd distkv-go
go version                      # 1.22 or newer
go test ./...                   # everything fails CLEANLY at first: no panics, no hangs. That is the starting state.
go test ./internal/kv -run Step01 -v
```

You should see **73 failing tests** across the seven steps, with messages like `not implemented yet: Step 0N: ...`.
That is the "red" in red-green-refactor. 🎯

Work through a step:

1. Read the step's PRD (`docs/steps/step-01-inmemory-store.md` — the current one; siblings are in `docs/steps/`) — especially the
   **Concept corner**.
2. Open the tests (`internal/<package>/<step>_test.go`) and read them **before** coding. They are the spec.
3. Open the stub (the doc comment on each stub repeats the requirements).
4. Run only that step while you work: `go test ./internal/kv -run Step01 -v`.
5. When green, read **Reflect**, try a **Stretch**, run `go vet ./...` and `gofmt -l .`, then compare with
   `_solutions/stepNN/…`. The differences are where the learning is.
6. `git commit`, next step.

**Stuck?** Hints in each PRD go from gentle to explicit. As a last resort:

```bash
bash scripts/apply-solution.sh 01            # macOS / Linux / Git-Bash
.\scripts\apply-solution.ps1 01              # Windows PowerShell
```

> Note: steps build on earlier ones. Step 03 replica tests need Step 01 to pass; Step 05 (like Step 03) needs both.
> Step 06's `recovery` tests need Step 01 (through 05). Step 07's `snapshot` tests need Steps 01–06. Apply in order:
> `apply-solution.sh 01 && apply-solution.sh 05 && apply-solution.sh 06 && apply-solution.sh 07` etc.

## Roadmap

| # | Package | Feature | Go ideas | Patterns / principles | Tests |
|---|---------|---------|----------|-----------------------|:------:|
| 01 | `kv` | [The single-node store](docs/steps/step-01-inmemory-store.md) | `sync.RWMutex`, `context`, sentinel errors, `%w`, `errors.Is` | concurrency, error handling | 6 |
| 02 | `wal` | [The write-ahead log: durability and crash recovery](docs/steps/step-02-write-ahead-log.md) | `encoding/binary`, `hash/crc32`, `fsync`, torn-tail recovery | durability, WAL | 16 |
| 03 | `replica` | [Replication: leader + follower, quorum writes](docs/steps/step-03-replication.md) | leader/follower, quorum, rollback | replication | 11 |
| 04 | `election` | [Leader election: terms, votes, heartbeats](docs/steps/step-04-election.md) | terms, majority votes, state machine | consensus, fault tolerance | 10 |
| 05 | `raft` | [Raft in miniature: log replication, commit index, apply](docs/steps/step-05-log-replication.md) | log matching, commit index, apply | Raft, consensus | 10 |
| 06 | `recovery` | [Crash recovery: atomic save, verified load, replay](docs/steps/step-06-recovery.md) | `encoding/json`, `fsync`, rename, replay-through-the-real-path | atomic writes, fail-closed | 10 |
| 07 | `snapshot` | [Snapshots: compact the applied state, keep the suffix](docs/steps/step-07-snapshot.md) | absolute log positions, snapshot boundary | snapshotting | 10 |

**73 tests in total.** Steps 1, 2 and 4 stand alone. Step 3 builds on 1. Step 5 builds on 1. Step 6 builds on 1, 2 and 5. Step 7 builds on 1, 5 and 6.

## What you'll learn, in order

1. **Step 01 — the single node:** mutexes, contexts, error values. The foundation everything else stands on. ✅
2. **Step 02 — durability:** write-ahead logs, fsync, crash recovery, torn tails. ✅
3. **Step 03 — replication:** leader, followers, quorum writes, rollback on failed commit — and the divergence you create. ✅
4. **Step 04 — elections:** terms, votes, heartbeats — surviving the leader's death. ✅
5. **Step 05 — consensus in miniature:** log replication, commit index, apply, linearizable state machine. ✅
6. **Step 06 — crash recovery:** atomic save, verify-on-load, replay through the real AppendEntries path. ✅
7. **Step 07 — snapshots:** compact the applied state, keep the uncommitted suffix, restore a node that remembers its boundary. ✅

Plus stretches: snapshot install for lagging followers, real networking between two processes, membership change (joint consensus), and re-applying the archive after the suffix is gone.

## Project layout

```
distkv-go/
├── README.md
├── go.mod
├── docs/
│   └── steps/             # ← the PRDs, one per step
├── internal/
│   ├── todo/              # ErrNotImplemented for stubs
│   ├── kv/                # step 01: the single-node store
│   ├── wal/               # step 02: the write-ahead log
│   ├── replica/           # step 03: replication (leader + followers, quorum writes)
│   ├── election/          # step 04: Raft-style election (terms, votes, heartbeats)
│   ├── raft/              # step 05: Raft — log replication and the commit index
│   ├── recovery/          # step 06: crash recovery (save, verify, replay)
│   └── snapshot/          # step 07: snapshots (compact applied state, keep the suffix)
├── _solutions/            # reference solutions (ignored by the Go tool)
│   └── step01/ … step07/
└── scripts/               # apply-solution.sh / apply-solution.ps1
```

**Why `_solutions`?** The Go tool ignores directories whose names start with `_`, so `go test ./...` and `go vet ./...` never try to compile the reference solutions as if they were packages.

## Ground rules that make this effective

- Don't edit the tests to make them pass. If a test looks wrong, that is a great question — read the PRD again, then ask.
- Resist peeking at `_solutions/` until your tests are green.
- Run `go vet ./...` and `gofmt -l .` before every commit.
- Keep the **Reflect** answers in a notes file. Explaining *why* in your own words is what makes it stick.

## How this project was built

- Starting state: all **73 step tests fail cleanly** — no panics, no hangs, no compile errors in the stub packages.
- Solved state (verified with `_solutions/step01`…`_solutions/step07` applied cumulatively): **all 73 pass under `-race`** with `go vet` and `gofmt` clean.
- The reference solutions are in `_solutions/step01/` through `_solutions/step07/`.
