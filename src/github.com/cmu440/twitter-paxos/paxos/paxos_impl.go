// Implementation for paxos
package paxos

import (
//"encoding/json"
//"fmt"
//"io/ioutil"
//"log"
//"net"
//"net/http"
//"net/rpc"
//"sort"
//"strconv"
//"time"

//"github.com/cmu440/tribbler/libstore"
//"github.com/cmu440/tribbler/rpc/tribrpc"
)

// Phase is either Prepare or Accept
type Phase int

const (
	Prepare Status = iota + 1
	Accept
)

type paxosStates struct {
	isLeader             bool
	numNodes, numAccepts int
	phase                Phase
	nodes                []string
	myPort               int
    myProposal          proposal
}

type proposal struct {
	port, seqNum int
	key, value   string
}

// This function creates a paxos package for storage server to use.
// nodes is an array of host:port strings
func NewPaxos(myPort int, nodes []string) (PaxosStates, error) {
	pState := &paxosStates{isLeader: false, nodes: nodes, numNodes: len(nodes)}
	pState.myPort = myPort

	return pState
}

// This function is used by a node to make a new proposal
// to other nodes. It will return true when a proposal is
// being successfully made. Otherwise, it will return false.
func (ps *paxosStates) Prepare(key, val string) (bool, error) {
	prop := &proposal{port: ps.myPort, key: key, val: val}
	prop.seqNum = time.Now().UnixNano()

	// iter := 0
	// exp := 1
	for {

		//send

		<-time.after(time.Second)
		// wait with exponential back-off using iter
		// <-time.after(time.Duration(exp) * time.Second)
		// iter++
		// exp *= 2
	}
}

// This function is used by a leader to do all the work in
// accept phase. It will return true when a majority of nodes
// reply accept-ok. Otherwise, it will return false.
func (ps *paxosStates) Accept() (bool, error) {
}

// This function is used by a leader to commit a value after
// a successful accept phase.
func (ps *paxosStates) commitVal(prop proposal) error {
}

// This function is used by a leader upon receiving a prepare
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) ReceivePrepareResponse() {
}

// This function is used by a leader upon receiving an accept
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) ReceiveAcceptResponse() {
}

// This function is used by an acceptor to react to a prepare
// message. It will react corresponding to the current state.
func (ps *paxosStates) ReceivePropose() {
}

// This function is used by an acceptor to react to an accept
// message. It will react corresponding to the current state.
func (ps *paxosStates) ReceiveAccept() {
}
