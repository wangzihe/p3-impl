// Implementation for paxos
package paxos

import (
    "encoding/json"
    "errors"
//"fmt"
//"io/ioutil"
//"log"
//"net"
//"net/http"
//"net/rpc"
//"sort"
    "strconv"
    "strings"
    "sync"
    "time"

//"github.com/cmu440/tribbler/libstore"
//"github.com/cmu440/tribbler/rpc/tribrpc"
)

// Phase is either Prepare or Accept
type Phase int

const (
    None = iota
	Prepare
	Accept
    PrepareReply
    AcceptReply
)

type paxosStates struct {
    // the following are constant once NewPaxos is called
    nodes                       []string    // list of all HostPorts
    myHostPort                  string
	myID, numNodes              int

    accedMutex                  *sync.Mutex // protects the following variables
	numAcc, numRej    int
	phase                       Phase

    accingMutex                 *sync.Mutex // protects the following variables
    n_h                         int     // highest seqNum seen so far
    n_a                         int     // highest seqNum accepted; -1 if none
    v_a                         string  // value associated with n_a
    mySeqNum                    int     // seqNum of my proposal if proposing

    accChan                     chan bool // channel for reporting quorum
}

type p_message struct {
    mtype        Phase
	HostPort    string
    seqNum      int
    acc         bool
    val         string
}

// This function creates a paxos package for storage server to use.
// nodes is an array of all host:port strings; this needs to be identical for
// each call to newPaxosStates (i.e., for each server), because it is used to
// generate the node's ID number, which in turn ensures that sequence numbers
// are unique to that node
func NewPaxosStates(myHostPort string, nodes []string) (PaxosStates, error) {
	ps := &paxosStates{nodes: nodes, numNodes: len(nodes)}
    for i := 0; nodes[i] != myHostPort; i++ {
    }
    myID = i

	return ps, nil
}

// This function is used by a node to make a new proposal
// to other nodes. It will return true when a proposal is
// being successfully made. Otherwise, it will return false.
func (ps *paxosStates) Prepare(HostPorts []string, msg []byte) (bool, error) {

    // set phase
    ps.accedMutex.Lock()
    ps.phase = Prepare
    ps.accedMutex.Unlock()

    for i := 0; i < len(ps.nodes); i++ {
	    // send marshalled to ith node
    }

    toReturn := false

    select {
	case <-time.After(time.Second): // for now, just time out after 1 second
    case acc := <-ps.accChan:
        if acc {
            // TODO: anything else?
            toReturn = true
        }
    }

    return toReturn, nil
}

// This function is used by a leader to do all the work in
// accept phase. It will return true when a majority of nodes
// reply accept-ok. Otherwise, it will return false.
func (ps *paxosStates) Accept(key, val string) (bool, error) {
    return false, nil
}

// This function is used by a leader to commit a value after
// a successful accept phase.
func (ps *paxosStates) commitVal(msg p_message) error {
    return nil
}

// unmarshalls and determines the type of a p_message
// and calls the appropriate handler function
func (ps *paxosStates) Interpret_message(marshalled []byte) error {
    var msg p_message
	err = json.Unmarshal(marshalled, &msg)
	if err != nil {
		return err
	}
    switch msg.mtype
    case None:
        return errors.New("Invalid msg state: None")
	case Prepare:
        ps.receiveProposal(msg)
	case Accept:
        ps.receiveAcceptReq(msg)
    case PrepareReply:
        ps.receivePrepareResponse(msg)
    case AcceptReply:
        ps.receiveAcceptResponse(msg)
}

// This function is used by a leader upon receiving a prepare
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) receivePrepareResponse(msg p_message) {
    ps.accedMutex.Lock()
    if ps.phase == Prepare {
        if ps.mySeqNum == msg.seqNum {
           if msg.acc {
               ps.numAcc++
           } else {
               ps.numRej++
           }
        }
        if 2*ps.numAcc > ps.numNodes { // proposal accepted
            ps.accChan <- true
        } else if 2*ps.numRej > ps.numNodes { // proposal rejected
            ps.accChan <- false
        }
    }
    ps.accedMutex.Unlock()
}

// This function is used by a leader upon receiving an accept
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) receiveAcceptResponse(msg p_message) {
    return
}

// This function is used by an acceptor to react to a prepare
// message. It will react corresponding to the current state.
func (ps *paxosStates) receiveProposal(msg p_message) {
    resp := &p_message{HostPort: ps.myHostPort}
    ps.accingMutex.Lock()
    if msg.seqNum > ps.n_h { // accept
        resp.acc = true
        resp.val = ps.v_a
        resp.seqNum = ps.n_a
        ps.n_h = msg.seqNum
    } else { // reject
        resp.acc = false
    }
    ps.accingMutex.Unlock()
}

// This function is used by an acceptor to react to an accept
// message. It will react corresponding to the current state.
func (ps *paxosStates) receiveAccept(msg p_message) error {
    if ps.n_h > msg.seqNum {  // reject
        reply := &p_message{mtype: AcceptReply, acc: false}
    } else {    // accept
        ps.n_a = msg.seqNum
        ps.v_a = msg.val
        ps.n_h = msg.seqNum
        reply := &p_message{mtype: AcceptReply, acc: true}
    }
    // TODO: send reply the msg.HostPort
    marshalled, err := json.Marshal(msg)
    if err
}

// returns a marshalled Prepare p_message with (hopefully) a highest new seqNum
func (ps *paxosStates) CreatePrepareMsg() ([]byte, error) {
    // generate a sequence number, ensured to be unique by
    // taking next multiple of numNodes and adding myID
    ps.accingMutex.Lock()
	ps.n_h = ps.numNodes*(1 + (n_h/ps.numNodes)) + ps.myID
    ps.mySeqNum = ps.n_h
	msg := &p_message{mtype: Prepare, HostPort: ps.myHostPort, seqNum: ps.n_h}
    ps.accingMutex.Unlock()

	return json.Marshal(msg)
}

// returns a marshalled Accept p_message with value val and my proposal seqNum
func (ps *paxosStates) CreateAcceptMsg(val string) ([]byte, error) {
    ps.accingMutex.Lock()   // probably uneccesary (for seqNum)
	msg := &p_message{mtype: Accept, HostPort: ps.myHostPort, seqNum: ps.mySeqNum, val: val}
    ps.accingMutex.Unlock()
	return json.Marshal(msg)
}
