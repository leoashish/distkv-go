# Step 01 — The single-node store: `Get`, `Put`, `Delete`

| | |
|---|---|
| **Build** | `internal/kv/store.go` — the three methods of `kv.Store` |
| **Tests** | `internal/kv/kv_test.go` — 6 tests |
| **Run** | `go test ./internal/kv -run Step01 -v` |
| **You'll learn** | `sync.RWMutex`, always-`…Context` methods, sentinel errors + `%w` wrapping, `errors.Is` |
| **Time** | ~45–60 min |

## 1. The story

DistKV is a distributed key-value store. But a distributed store is nothing without a **node** that holds state in the first place.

Step 01 builds that node. A single node is a single machine, a single process, one in-memory map from string keys to string values. Goroutines — HTTP handlers, background workers, a test fan-out — call `Get`/`Put`/`Delete` on it, possibly at the same time.

The *distribution* part (write-ahead log, leader, followers, elections, replicated log) comes in later steps. Step 01 builds the brain that those steps will put behind a log file and a network. Everything you do here is going to be wrapped later; for now, plain memory is honest and simple.

## 2. Concept corner

- **There is no `async/await` in Go.** Go code *blocks*, and goroutines (cheap: thousands at a time) make blocking fine. While one goroutine waits, the runtime runs others. So `store.Get(ctx, key)` just returns the value — no task, no await. Concurrency is something *callers* opt into with `go`; you start doing it properly from this very step.
- **`sync.RWMutex` — many readers, one writer.** A plain `sync.Mutex` would be correct too; `RWMutex` lets parallel reads run concurrently. This is your first distributed-systems-adjacent decision: *what does the state look like under concurrency?*
- **Always the `…Context` methods.** `Get(ctx, …)`, `Put(ctx, …)`, `Delete(ctx, …)`. The context is how callers cancel a slow operation or set a deadline; in a distributed store it is also how a follower notices a leader is gone. Pass the ctx you were given; never invent `context.Background()` inside the store.
- **Errors are values.** "Key not found" is not an exception; it is a sentinel (`kv.ErrNotFound`). Translate/define it, and **wrap** with `%w` so callers can ask `errors.Is(err, kv.ErrNotFound)` *and* read context off the message.

## 3. Requirements

Implement the three methods of `kv.Store`:

| Method | Behaviour |
|---|---|
| `Get(ctx, key) (string, error)` | The value stored under `key`, or an error satisfying `errors.Is(err, kv.ErrNotFound)`. |
| `Put(ctx, key, value) error` | Stores `value` under `key`, overwriting whatever was there. |
| `Delete(ctx, key) error` | Removes `key`. Deleting a key that is not there is a **no-op success** (idempotent). |

Cross-cutting rules every method must follow:

- Every method returns an error wrapping `context.Canceled` when given an already-cancelled context.
- The store must be safe for concurrent use — many readers, writers in parallel (run the tests with `-race`).
- A missing key for `Get` is an error (`ErrNotFound`); a missing key for `Delete` is not.

## 4. What the tests pin down

- `Put` then `Get` returns the value.
- `Get` of an unknown key fails with `errors.Is(err, kv.ErrNotFound)`.
- `Delete` removes the key (subsequent `Get` misses).
- `Delete` of a missing key is a no-op success — twice in a row.
- All three methods with an already-cancelled context return `context.Canceled`.
- 20 goroutines doing puts + gets all succeed without losing data (run with `-race`).

## 5. Hints (open one at a time)

<details><summary>Hint 1 — the shape of every method</summary>

Each method starts by checking the context, takes the appropriate lock, reads/writes the map, and unlocks (via `defer`). In-memory operations have no ctx-aware API of their own, so checking `ctx.Err()` yourself is how cancellation reaches the operation.

</details>

<details><summary>Hint 2 — RWMutex or Mutex?</summary>

`sync.RWMutex`: many concurrent readers, one writer at a time. `Get` takes the **R** lock, `Put`/`Delete` take the **write** lock, always released via `defer`.

</details>

<details><summary>Hint 3 — "no such key" is a sentinel</summary>

`var ErrNotFound = errors.New("not found")` in the package; for `Get` miss: `fmt.Errorf("kv: get %q: %w", key, ErrNotFound)`. `errors.Is` will then work through the wrap.

</details>

<details><summary>Hint 4 — cancelled context</summary>

`if err := ctx.Err(); err != nil { return ..., err }` — or wrap it. It must surface as `context.Canceled` via `errors.Is`.

</details>

## 6. Stretch challenges

1. Make `Delete` strict: deleting a missing key returns `ErrNotFound`. Which behaviour do real KV stores (Redis, etcd) actually have?
2. Add `Size() int` — total number of keys. (A distributed store will need this for compaction.)
3. Add a `Keys(prefix string) []string` listing keys — preparation for future range scans.

## 7. Reflect

- Why is an in-memory map + mutex enough for a *single* node, and why will later steps need more than that?
- Why must every method take the context as first parameter, even when the operation is "instant"?
- What would break if you used a plain `sync.Mutex` for reads too? (Hint: throughput.)
- Why wrap the sentinel with `%w` instead of just returning it?

## 8. Done when

`go test ./internal/kv -run Step01 -race -v` → **ok**. Then `go vet ./...` and `gofmt -l .`, and compare with `_solutions/step01/…`.

## 9. Next

Step 02 — the write-ahead log: durability (append, `fsync`, reload, torn-tail recovery). Its PRD will be written when you finish this one — the pattern is fully established now, so don't wait for future-you to finish it.
