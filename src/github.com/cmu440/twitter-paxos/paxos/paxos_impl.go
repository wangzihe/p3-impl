// Implementation for paxos
package paxos

import (
    "encoding/json"
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
	myPort, numNodes            int

    accedMutex                  *sync.Mutex // protects the following variables
	numAcc, numRej    int
	phase                       Phase
    acced                       map[string]bool // if HostPort accepted prop

    accingMutex                 *sync.Mutex // protects the following variables
    n_h                         int     // highest seqNum seen so far
    n_a                         int     // highest seqNum accepted; -1 if none seen
    v_a                         string  // value associated with n_a

    accChan                     chan bool // channel for reporting quorum
}

type p_message struct {
    p           Phase
	HostPort    string
    seqNum      int
    acc         bool
    val         string
}

// This function creates a paxos package for storage server to use.
// nodes is an array of host:port strings
func NewPaxosStates(myHostPort string, nodes []string) (PaxosStates, error) {
	ps := &paxosStates{nodes: nodes, numNodes: len(nodes)}
    var err error
	ps.myPort, err = strconv.Atoi(strings.Split(myHostPort,":")[1])
    if err != nil {
        return nil, err
    }

	return ps, nil
}

// This function is used by a node to make a new proposal
// to other nodes. It will return true when a proposal is
// being successfully made. Otherwise, it will return false.
func (ps *paxosStates) Prepare(HostPorts []string, msg []byte) (bool, error) {

    // generate a sequence number, ensured to be unique
    ps.accingMutex.Lock()
	// prop.seqNum = ps.numNodes*(1 + (n_h/ps.numNodes)) + ps.myPort
    // TODO: move this to createPrepareMsg ps.n_h = prop.seqNum
    ps.accingMutex.Unlock()

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

func (ps *paxosStates) Interpret_message(marshalled []byte) error {
    return nil
}

// This function is used by a leader upon receiving a prepare
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) receivePrepareResponse(msg p_message) {
    ps.accedMutex.Lock()
    if ps.phase == Prepare {
        // TODO: if ps.myProp.seqNum == msg.seqNum {
            if _, pres := ps.acced[msg.HostPort]; !pres {
                ps.acced[msg.HostPort] = msg.acc
                if msg.acc {
                    ps.numAcc++
                } else {
                    ps.numRej++
                }
            }
        // }
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
func (ps *paxosStates) receivePropose(msg p_message) {
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
func (ps *paxosStates) receiveAccept(msg p_message) {
    if ps.n_h > msg.seqNum {  // reject
    } else {    // accept
    }
}

func (ps *paxosStates) CreatePrepareMsg() ([]byte, error) {
	msg := &p_message{p: Prepare, HostPort: ps.myHostPort}
	return json.Marshal(msg)
}

func (ps *paxosStates) CreateAcceptMsg(val string) ([]byte, error) {
    return nil, nil
}
