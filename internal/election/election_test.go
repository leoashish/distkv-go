package election_test

import (
	"testing"

	"distkv/internal/election"
)

// Step 04 -- leader election.   Run:  go test ./internal/election -run Step04 -v

func newNode(t *testing.T, id string) *election.Node {
	t.Helper()
	n := election.NewNode(id)
	if n == nil {
		t.Fatalf("NewNode(%q) returned nil", id)
	}
	return n
}

func vote(t *testing.T, n *election.Node, term uint64, candidate string) election.VoteResponse {
	t.Helper()
	return n.RequestVote(election.VoteRequest{Term: term, CandidateID: candidate})
}

func TestStep04_NewNode_StartsAsFollowerAtTermZero(t *testing.T) {
	n := newNode(t, "solo")

	if n.Term != 0 {
		t.Fatalf("Term = %d, want 0", n.Term)
	}
	if n.State != election.Follower {
		t.Fatalf("State = %v, want election.Follower", n.State)
	}
}

func TestStep04_SingleNode_ElectsItself(t *testing.T) {
	n := newNode(t, "solo")

	if !n.StartElection(nil) {
		t.Fatal("the only node in a 1-node cluster must be able to elect itself")
	}
	if n.State != election.Leader {
		t.Fatalf("State = %v, want election.Leader", n.State)
	}
}

func TestStep04_ThreeNodes_MajorityGrantsLeadership(t *testing.T) {
	a := newNode(t, "a")
	b := newNode(t, "b")
	c := newNode(t, "c")

	if !a.StartElection([]*election.Node{b, c}) {
		t.Fatal("a should win: b and c are free to vote, and 2 of 3 is a majority")
	}
	if a.State != election.Leader {
		t.Fatalf("State = %v, want election.Leader", a.State)
	}
	if a.Term != 1 {
		t.Fatalf("Term = %d, want 1 (the candidate bumped it)", a.Term)
	}
}

func TestStep04_ThreeNodes_FailsWhenMajorityPreVotedSomeoneElse(t *testing.T) {
	a := newNode(t, "a")
	b := newNode(t, "b")
	c := newNode(t, "c")

	// b and c vote for a rival candidate at the term a would reach for.
	vote(t, b, 1, "dummy")
	vote(t, c, 1, "dummy")

	if a.StartElection([]*election.Node{b, c}) {
		t.Fatal("a cannot win after b and c already spent their vote for this term")
	}
	if a.State == election.Leader {
		t.Fatal("a must not become leader without a majority")
	}
}

func TestStep04_FiveNodes_WinsWithTwoPreVotesAgainst(t *testing.T) {
	a := newNode(t, "a")
	b := newNode(t, "b")
	c := newNode(t, "c")
	d := newNode(t, "d")
	e := newNode(t, "e")

	// Two peers refuse (they already voted this term). Two still grant, making 3 votes in a 5-node cluster.
	vote(t, b, 1, "dummy")
	vote(t, c, 1, "dummy")

	if !a.StartElection([]*election.Node{b, c, d, e}) {
		t.Fatal("a should win: a + d + e = 3 of 5 is a majority, even with two refusals")
	}
	if a.State != election.Leader {
		t.Fatalf("State = %v, want election.Leader", a.State)
	}
}

func TestStep04_RequestVote_VotesForTheFirstRequesterOnly_WithinATerm(t *testing.T) {
	n := newNode(t, "voter")

	first := vote(t, n, 7, "candidate-1")
	second := vote(t, n, 7, "candidate-2")

	if !first.VoteGranted {
		t.Fatal("first requester in term 7 must get the vote")
	}
	if second.VoteGranted {
		t.Fatal("a node votes for at most one candidate per term: candidate-2 must be refused")
	}
	if second.Term != 7 {
		t.Fatalf("the refusal carries Term = %d, want 7 (the voter's term)", second.Term)
	}
}

func TestStep04_RequestVote_RejectsAStaleTerm(t *testing.T) {
	n := newNode(t, "voter")
	vote(t, n, 9, "someone")

	stale := vote(t, n, 4, "old-candidate")

	if stale.VoteGranted {
		t.Fatal("a vote request from a stale term must be rejected")
	}
	if n.Term != 9 {
		t.Fatalf("after a stale request, Term = %d, want 9 (unchanged)", n.Term)
	}
}

func TestStep04_Heartbeat_AdoptsAHigherTermAndReturnsToFollower(t *testing.T) {
	n := newNode(t, "node")
	if !n.StartElection(nil) { // becomes leader at term 1
		t.Fatal("setup: solo node must become leader")
	}

	ok := n.Heartbeat(7, "bigger-leader")

	if !ok {
		t.Fatal("Heartbeat(7) with a leader at term 7+ must be accepted")
	}
	if n.Term != 7 {
		t.Fatalf("Term = %d, want 7 (adopted from the heartbeat)", n.Term)
	}
	if n.State != election.Follower {
		t.Fatalf("State = %v, want election.Follower", n.State)
	}
}

func TestStep04_Heartbeat_RejectsALowerTerm(t *testing.T) {
	n := newNode(t, "node")
	vote(t, n, 9, "someone") // pushes the voter to term 9

	ok := n.Heartbeat(4, "old-leader")

	if ok {
		t.Fatal("a lower-term heartbeat must be rejected")
	}
	if n.Term != 9 {
		t.Fatalf("Term = %d, want 9 (unchanged)", n.Term)
	}
}

func TestStep04_FollowerVotesAreFreshAcrossTerms(t *testing.T) {
	n := newNode(t, "voter")

	vote(t, n, 3, "old-candidate")
	refreshed := vote(t, n, 4, "new-candidate")

	if !refreshed.VoteGranted {
		t.Fatal("across terms, a voter speaks fresh: it may vote again in the new term")
	}
	if n.Term != 4 {
		t.Fatalf("Term = %d, want 4", n.Term)
	}
}
