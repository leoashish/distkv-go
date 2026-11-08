# Step 04 — Leader election: terms, votes and heartbeats

| | |
|---|---|
| **Build** | `internal/election/ops.go` — `NewNode`, `Node.{RequestVote, StartElection, Heartbeat}` |
| **Given** | `State` (`Follower`, `Candidate`, `Leader`), `VoteRequest`, `VoteResponse`, `Node` (`internal/election/election.go`) |
| **Tests** | `internal/election/election_test.go` — 10 tests |
| **Run** | `go test ./internal/election -run Step04 -v` |
| **You'll learn** | Raft terms, one-vote-per-term, majority elections, heartbeats, stale-term rejection |
| **Time** | ~2 h |

## 1. The story

Step 03 gave DistKV a leader. But nothing stops a partitioned, stale leader from accepting writes forever — it has no idea it is not the leader anymore.

Step 04 gives the cluster the missing ingredient: **the ability to elect a new leader, safely**, by majority vote. No two nodes can hold leadership for the same term — because each node votes at most once per term, and two majorities of the same set *intersect*.

## 2. Concept corner

- **Terms**: monotonically increasing integers. Every leadership is scoped to a term; a candidate *starts* a new term by bumping it.
- **One vote per term per node**: a node grants its vote in a term to exactly one candidate — the first to ask. (Any further requests within the term are refused.)
- **Majority decides**: a majority of (cluster peers + the candidate itself) must grant the vote. For 3 nodes, 2; for 5, 3.
- **Votes are per-term**: once a term is over, the voting record resets — the same node may vote fresh in the next term.
- **Heartbeats**: a leader periodically announces itself at its term; any node whose term is ≤ the heartbeat adopts it and returns to Follower. A lower-term heartbeat is rejected.
- **A stale node learns too late**: when it finally hears of a higher term, it steps down — never able to claim authority it doesn't have. The response always carries the voter's current term so the candidate can adopt it.

## 3. Requirements

| Function | Behaviour |
|---|---|
| `NewNode(id string) *Node` | A follower at term 0. |
| `Node.RequestVote(req VoteRequest) VoteResponse` | Older term → reject, term unchanged. Newer term → move to that term, become Follower, clear the vote. Same term → vote for at most one candidate (refuse the second). Returns the voter's current term. |
| `Node.StartElection(others []*Node) bool` | Term +1, become Candidate, vote for self, ask every peer; collect grants; strictly more votes than half of (others + self) → Leader, true; else false, not leader (a failed election leaves you a Candidate). |
| `Node.Heartbeat(term uint64, leaderID string) bool` | term ≥ node's term → adopt it (higher moves term & resets vote), Role = Follower, true. Otherwise false, nothing changes. |

## 4. What the tests pin down

- New nodes are followers at term 0.
- A solo node elects itself; a 3-node cluster majority elects; 2 pre-votes of 3 fail; 2 pre-votes of 5 (with 2 fresh grantors) still win.
- One vote per term per voter: first requester wins; the second is refused (with the voter's term returned).
- Stale-term requests are rejected; the voter's term is untouched.
- A heartbeat at a higher term is adopted (term bumps, role → Follower); a lower-term heartbeat is rejected.
- After a term ends, the voter is free to vote fresh.

## 5. Hints (open one at a time)

<details><summary>Hint 1 — RequestVote</summary>

Three cases in order: `req.Term < n.Term` → refuse; `req.Term > n.Term` → adopt term, become follower, reset votedFor; then the vote check (`votedFor` empty or the same candidate).

</details>

<details><summary>Hint 2 — StartElection</summary>

`n.Term++; n.State = Candidate; n.votedFor = n.ID; votes := 1` — then ask every peer; if a peer reports a higher term, adopt it and step down with `false`. `votes > (len(others)+1)/2` → Leader.

</details>

<details><summary>Hint 3 — Heartbeat</summary>

`if term < n.Term { return false }`; otherwise adopt (reset vote if the term moved), set Follower, return true.

</details>

<details><summary>Hint 4 — the peer adoption dance</summary>

In StartElection, check `resp.Term > n.Term` and return false — the candidate learns immediately that it is behind and steps down instead of waiting out the doomed election.

</details>

## 6. Stretch challenges

1. **Split votes:** 4 nodes vote in a 2-2 tie. What makes sure a re-election (with randomised timeouts) eventually resolves it? Implement a randomised election timeout (`time.AfterFunc` or a loop).
2. **RequestVote reset:** when a node grants a vote but its cluster later discovers a higher-term node, verify the stale leader refuses all reads/writes automatically via terms.
3. **Candidate with stale term:** after a failed election at term T, the candidate learns of T+1; assert it adopts it.

## 7. Reflect

- Why does a node vote at most once per term, and why is that enough to prevent two leaders?
- What makes "the latest term wins" equivalent to "the one with the freshest log wins" up to a point?
- Why do heartbeat terms matter, and what happens if you ignore them?
- Why must a candidate bump its term even if it loses? (Otherwise the next election would reuse stale votes.)

## 8. Done when

`go test ./internal/election -run Step04 -v` → **ok**. Then `go vet ./...`, `gofmt -l .`, and compare with `_solutions/step04/…`.

## 9. Next

Step 05 — **the Raft log**: AppendEntries, the commit index, and applying committed entries to the state machine.
