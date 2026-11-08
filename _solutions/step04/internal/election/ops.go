package election

// REFERENCE SOLUTION for Step 04. Try it yourself first!

func NewNode(id string) *Node {
	return &Node{ID: id, State: Follower}
}

func (n *Node) RequestVote(req VoteRequest) VoteResponse {
	if req.Term < n.Term {
		return VoteResponse{Term: n.Term, VoteGranted: false}
	}
	if req.Term > n.Term {
		n.Term = req.Term
		n.votedFor = "" // the term moved on: the vote is fresh again
		n.State = Follower
	}
	if n.votedFor != "" && n.votedFor != req.CandidateID {
		return VoteResponse{Term: n.Term, VoteGranted: false}
	}
	n.votedFor = req.CandidateID
	return VoteResponse{Term: n.Term, VoteGranted: true}
}

func (n *Node) StartElection(others []*Node) bool {
	n.Term++
	n.State = Candidate
	n.votedFor = n.ID // a candidate votes for itself

	votes := 1
	for _, p := range others {
		resp := p.RequestVote(VoteRequest{Term: n.Term, CandidateID: n.ID})
		if resp.Term > n.Term { // the peer is ahead: adopt its term and step down
			n.Term = resp.Term
			n.State = Follower
			n.votedFor = ""
			return false
		}
		if resp.VoteGranted {
			votes++
		}
	}
	if votes > (len(others)+1)/2 {
		n.State = Leader
		return true
	}
	return false
}

func (n *Node) Heartbeat(term uint64, leaderID string) bool {
	if term < n.Term {
		return false
	}
	if term > n.Term {
		n.Term = term
		n.votedFor = ""
	}
	n.State = Follower
	return true
}
