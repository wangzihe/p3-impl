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
//"strconv"
//"time"

//"github.com/cmu440/tribbler/libstore"
//"github.com/cmu440/tribbler/rpc/tribrpc"
)

// Phase is either Prepare or Accept
type Phase int

const (
    None = iota
	Prepare
	Accept
)

type paxosStates struct {
	numNodes, numAcc, numrej    int
	phase                       Phase
	nodes                       []string
    acced                       map[string]bool // if HostPort accepted prop
    //
    accedMutex, accingMutex     *sync.Mutex
    accChan                     chan bool
	myPort                      int
    myHostPort                  string
    myProp                      proposal
    myAccs                      map[int]bool
    n_h                         int     // highest seqNum seen so far
    n_a                         int
    v_a                         string
}

type proposal struct {
    p Phase
	HostPort    string
    seqNum int
	key, value   string
}

type response struct {
	HostPort    string
    seqNum      int
	value  string
    acc         bool
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

	prop := &proposal{p: Prepare, HostPort: ps.myHostPort, key: key, val: val}

    // generate a sequence number, ensured to be unique
    accingMutex.Lock()
	prop.seqNum = ps.numNodes*(1 + (n_h/ps.numNodes)) + ps.myPort
    // an alternative: = time.Now().UnixNano()*ps.numNodes + ps.myPort
    ps.n_h = prop.seqNum
    accingMutex.Unlock()

    // set phase
    accedMutex.Lock()
    ps.phase = Prepare
    accedMutex.Unlock()

	marshalled, err := json.Marshal(prop)
	if err != nil {
		return false, err
	}
    for i := 0; i < len(ps.nodes) {
	    // send marshalled to ith node
    }

    toReturn = false

    select {
	case _ <-time.after(time.Second): // for now, just time out after 1 second
    case acc <- ps.accChan:
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
}

// This function is used by a leader to commit a value after
// a successful accept phase.
func (ps *paxosStates) commitVal(prop proposal) error {
}

// This function is used by a leader upon receiving a prepare
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) ReceivePrepareResponse(resp response) {
    ps.acceptedMutex.Lock()
    if ps.phase == Prepare {
        if ps.myProp.seqNum == r.seqNum {
            if _, pres := ps.accepted[r.HostPort]; !pres {
                ps.accepted[r.HostPort] = resp.acc
                if resp.acc {
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
    ps.acceptedMutex.Unlock()
}

// This function is used by a leader upon receiving an accept
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) ReceiveAcceptResponse() {
}

// This function is used by an acceptor to react to a prepare
// message. It will react corresponding to the current state.
func (ps *paxosStates) ReceivePropose(prop proposal) {
    resp := &response{HostPort: ps.HostPort}
    accingLock.Lock()
    if proposal.seqNum > ps.n_h { // accept
        resp.acc = true
        resp.value = v_a
        resp.seqNum = n_a
        ps.n_h = proposal.seqNum
    } else { // reject
        resp.acc = false
    }
    accingLock.Unlock()
}

// This function is used by an acceptor to react to an accept
// message. It will react corresponding to the current state.
func (ps *paxosStates) ReceiveAccept(prop proposal) {
    if n_h > prop.seqNum {
    }
}
