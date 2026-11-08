package election // STEP 04: implement leader election. Read docs/steps/step-04-election.md first.   Run:  go test ./internal/election -run Step04

// NewNode returns a follower with Term 0.
func NewNode(id string) *Node {
	return nil // TODO Step 04
}

// RequestVote handles a vote request.
//
// A request with a term OLDER than the voter's is rejected (the voter's term is unchanged).
// A request with a NEWER term moves the voter to that term, makes it a Follower, and clears its vote for that term.
// Inside a term, a node votes for at most one candidate — the first it heard from.
// The voter returns its CURRENT term so the candidate can learn whether it has been left behind.
func (n *Node) RequestVote(req VoteRequest) VoteResponse {
	return VoteResponse{} // TODO Step 04 -- tests fail loudly until you implement this
}

// StartElection runs a full election: the candidate bumps its term, votes for itself, asks every peer in
// others[], and becomes Leader iff a strict majority of (others + itself) voted for it.
// Returns true iff it became leader. A failed election returns false and leaves the node a Candidate.
func (n *Node) StartElection(others []*Node) bool {
	return false // TODO Step 04
}

// Heartbeat lets a leader announce itself: a node with Term <= the heartbeat's term adopts that term
// and returns to Follower (true); a lower-term heartbeat is rejected (false).
func (n *Node) Heartbeat(term uint64, leaderID string) bool {
	return false // TODO Step 04
}
