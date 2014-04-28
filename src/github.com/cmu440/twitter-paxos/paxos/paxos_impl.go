// Implementation for paxos
package paxos

import (
	"encoding/json"
	//"fmt"
	//"io/ioutil"
	"log"
	"net"
	//"net/http"
	//"net/rpc"
	//"sort"
	//"strconv"
	//"strings"
	"container/list"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"sync"

	"github.com/cmu440/twitter-paxos/message"
)

// Phase is either Prepare or Accept
type Phase int

const (
	None Phase = iota + 1 // Indicates the server is an acceptor
	Prepare
	Accept
	PrepareReply
	AcceptReply
	Commit
)

type paxosStates struct {
	// the following are constant once NewPaxos is called
	nodes          *list.List // list of all HostPorts
	myHostPort     string
	myID, numNodes int

	accedMutex     *sync.Mutex // protects the following variables
	numAcc, numRej int         // number of ok/reject response received
	phase          Phase       // which stage of paxos the server is at

	accingMutex *sync.Mutex // protects the following variables
	n_h         int         // highest seqNum seen so far; 0 if none has seen
	n_a         int         // highest seqNum accepted; -1 if none
	v_a         string      // value associated with n_a
	prep_v      string      // latest value received during prepare stage
	prep_n      int         // largest proposal ID received during prepare stage
	mySeqNum    int         // seqNum of my proposal if proposing

	prepChan   chan bool          // channel for reporting quorum during prepare phase
	accChan    chan bool          // channel for reporting quorum during accept phase
	logger     *log.Logger        // logger for paxos (same as the one for server)
	msgHandler message.MessageLib // message handler
	storage    *sql.DB            // database used by storage server
}

type p_message struct {
	mtype    Phase
	HostPort string // for sending response
	seqNum   int    // proposal ID corresponding to val
	acc      bool   // True if reply OK
	val      string // value related to the proposal. emptry string is there is no value to send
}

// This function creates a paxos package for storage server to use.
// nodes is an array of all host:port strings; this needs to be identical for
// each call to newPaxosStates (i.e., for each server), because it is used to
// generate the node's ID number, which in turn ensures that sequence numbers
// are unique to that node
func NewPaxosStates(myHostPort string, nodes *list.List,
	logger *log.Logger, storage *sql.DB) PaxosStates {
	ps := &paxosStates{nodes: nodes, myHostPort: myHostPort, numNodes: nodes.Len(), storage: storage}
	var i int = 0
	for e := nodes.Front(); e != nil; e = e.Next() {
		port := e.Value.(string)
		if port == myHostPort {
			break
		} else {
			i += 1
		}
	}
	ps.myID = i

	ps.accedMutex = &sync.Mutex{}
	ps.accingMutex = &sync.Mutex{}
	ps.numAcc = 0
	ps.numRej = 0
	ps.phase = None
	ps.n_h = 0
	ps.n_a = -1
	ps.v_a = ""
	ps.prep_v = ""
	ps.prep_n = -1
	ps.prepChan = make(chan bool)
	ps.accChan = make(chan bool)
	ps.logger = logger
	ps.msgHandler = message.NewMessageHandler()

	return ps
}

// This function is used by a node to make a new proposal
// to other nodes. It will return true when a proposal is
// being successfully made. Otherwise, it will return false.
func (ps *paxosStates) Prepare() (bool, error) {
	// set phase
	ps.accedMutex.Lock()
	if ps.phase != None {
		// server is busy handling the previous commit
		ps.accedMutex.Unlock()
		return false, nil
	}
	ps.phase = Prepare
	ps.accedMutex.Unlock()

	// create prepare message
	msgB, err := ps.CreatePrepareMsg()
	if err != nil {
		ps.logger.Printf("Prepare: error while creating prepare message. %s\n", err)
		return false, err
	}
	// send prepare message to the network
	ps.broadCastMsg(msgB)

	// wait for the majority to respond. Note that paxos doesn't
	// guarantee liveness. Therefore, we might wait forever due
	// to message loss.
	toReturn := false
	acc := <-ps.prepChan
	if acc {
		ps.logger.Printf("Prepare: prepare-ok from majority\n")
		toReturn = true
	}

	return toReturn, nil
}

// This function returns a marshalled Prepare p_message with
// (hopefully) a highest new seqNum
func (ps *paxosStates) CreatePrepareMsg() ([]byte, error) {
	// generate a sequence number, ensured to be unique by
	// taking next multiple of numNodes and adding myID
	ps.accingMutex.Lock()
	ps.n_h = ps.numNodes*(1+(ps.n_h/ps.numNodes)) + ps.myID
	ps.mySeqNum = ps.n_h
	msg := &p_message{mtype: Prepare, HostPort: ps.myHostPort, seqNum: ps.n_h}
	ps.accingMutex.Unlock()
	msgB, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, msgB)
	if err != nil {
		return nil, err
	}

	return generalMsg, nil
}

// This function is used by an acceptor to react to a prepare
// message. It will react corresponding to the current state.
func (ps *paxosStates) receiveProposal(msg p_message) {
	resp := &p_message{mtype: PrepareReply, HostPort: ps.myHostPort}
	ps.accedMutex.Lock()
	if ps.phase == None {
		// this server is acceptor
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
	} else {
		// this server is leader. Prevent two leaders in one system
		// leader must reject a prepare request of another node
		resp.acc = false
	}
	ps.accedMutex.Unlock()

	// send out response message
	msgB, err := json.Marshal(resp)
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, msgB)
	if err != nil {
		ps.logger.Printf("receiveProposal: error creating message. %s\n", err)
	} else {
		err = ps.sendMsg(msg.HostPort, generalMsg)
		if err != nil {
			ps.logger.Printf("receiveProposal: error sending message. %s\n", err)
		}
	}
}

// This function is used by a leader upon receiving a prepare
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) receivePrepareResponse(msg p_message) {
	ps.accedMutex.Lock()
	if ps.phase == Prepare {
		if msg.seqNum == ps.mySeqNum { // ignore responses to old requests
			if msg.acc {
				ps.numAcc++
				// udpate the uncommitted value and corresponding proposal ID
				if msg.val != "" {
					ps.accingMutex.Lock()
					if msg.seqNum > ps.prep_n {
						ps.prep_n = msg.seqNum
						ps.prep_v = msg.val
					}
					ps.accingMutex.Unlock()
				}
			} else {
				ps.numRej++
			}

			if 2*ps.numAcc > ps.numNodes { // proposal accepted
				ps.prepChan <- true
			} else if 2*ps.numRej > ps.numNodes { // proposal rejected
				ps.prepChan <- false
			}
		}
	}
	ps.accedMutex.Unlock()
}

// This function is used by a leader to do all the work in
// accept phase. It will return true when a majority of nodes
// reply accept-ok. Otherwise, it will return false.
func (ps *paxosStates) Accept(val string) (bool, error) {
	// set phase
	ps.accedMutex.Lock()
	ps.phase = Accept
	ps.numAcc = 0 // reset numAcc
	ps.numRej = 0 // reset numRej
	ps.accedMutex.Unlock()

	// if there are value not yet committed, choose the most recent
	// one. Since there is no other thread checking prev_v and prev_n,
	// therefore we don't grab lock
	var accept_val string
	if ps.prep_v != "" {
		// there are values not yet committed
		accept_val = ps.prep_v
	} else {
		// pick the value that client wants to commit
		accept_val = val
	}

	// create accept message
	msgB, err := ps.CreateAcceptMsg(accept_val)
	if err != nil {
		ps.logger.Printf("Accept: error while creating accept message. %s\n", err)
		return false, err
	}

	// send accept message to the network
	ps.broadCastMsg(msgB)

	// wait for the majority to respond. Note that paxos doesn't
	// guarantee liveness. Therefore, we might wait forever due
	// to message loss.
	toReturn := false
	acc := <-ps.accChan
	if acc {
		// majority accepts
		ps.logger.Printf("Accept: accept-ok from majority")
		toReturn = true
		// update v_a and n_a. Since we are the only thread updating,
		// no need to grab lock
		ps.v_a = accept_val
		ps.n_a = ps.mySeqNum
	} else {
		// majority rejects. reset paxos state
		ps.prep_v = ""
		ps.prep_n = -1
		ps.accedMutex.Lock()
		ps.phase = None
		ps.numAcc = 0
		ps.numRej = 0
		ps.accedMutex.Unlock()
	}

	return toReturn, nil
}

// This function returns a marshalled Accept p_message with value val
// and my proposal seqNum.
func (ps *paxosStates) CreateAcceptMsg(val string) ([]byte, error) {
	// No need to grab lock since no other thread will update ps.mySeqNum
	msg := &p_message{mtype: Accept, HostPort: ps.myHostPort, seqNum: ps.mySeqNum, val: val}
	msgB, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, msgB)
	if err != nil {
		return nil, err
	}

	return generalMsg, nil
}

// This function is used by an acceptor to react to an accept request.
// It will react corresponding to the current state.
func (ps *paxosStates) receiveAccept(msg p_message) {
	resp := &p_message{mtype: AcceptReply, HostPort: ps.myHostPort, seqNum: msg.seqNum}
	ps.accedMutex.Lock()
	if ps.phase == None {
		// this server is acceptor
		ps.accingMutex.Lock()
		if msg.seqNum >= ps.n_h { // accept
			resp.acc = true
			// update v_a and n_a
			ps.v_a = msg.val
			ps.n_a = msg.seqNum
		} else { // reject
			resp.acc = false
		}
		ps.accingMutex.Unlock()
	} else {
		// this server is leader. Prevent two leaders in one system
		// leader must reject an accept request of another node
		resp.acc = false
	}
	ps.accedMutex.Unlock()

	// send out response message
	msgB, err := json.Marshal(resp)
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, msgB)
	if err != nil {
		ps.logger.Printf("receiveProposal: error creating message. %s\n", err)
	} else {
		err = ps.sendMsg(msg.HostPort, generalMsg)
		if err != nil {
			ps.logger.Printf("receiveProposal: error sending message. %s\n", err)
		}
	}
}

// This function is used by a leader upon receiving an accept
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) receiveAcceptResponse(msg p_message) {
	ps.accedMutex.Lock()
	if ps.phase == Accept {
		if msg.seqNum == ps.mySeqNum { // ignore old responses
			if msg.acc {
				ps.numAcc++
			} else {
				ps.numRej++
			}

			if 2*ps.numAcc > ps.numNodes { // majority accepts
				ps.accChan <- true
			} else if 2*ps.numRej > ps.numNodes { // majority rejects
				ps.accChan <- false
			}
		}
	}

	ps.accedMutex.Unlock()
}

// This function is used by a leader to commit a value after
// a successful accept phase.
func (ps *paxosStates) commitVal() error {
	// construct commit message
	msgB, err := ps.CreateCommitMsg()
	if err != nil {
		return err
	}

	// send prepare message to the network
	ps.broadCastMsg(msgB)

	// commit on its local machine. since this is the only thead
	// reading v_a, therefore no need to grab the lock.
	tx, err := ps.storage.Begin()
	if err != nil {
		ps.logger.Printf("receiveCommit: database error. %s\n", err)
	} else {
		stmt, err := tx.Prepare("insert into storage(tweet, count) values(?, ?)")
		if err != nil {
			ps.logger.Printf("receiveCommit: database error. %s\n", err)
		}
		defer stmt.Close()
		if err != nil {
			_, err := stmt.Exec(ps.v_a, 1)
			if err != nil {
				ps.logger.Printf("receiveCommit: exec error. %s\n", err)
			}
		}
		tx.Commit()
	}

	// reset phase variable to None
	ps.prep_v = "" // no need to grab lock since we are the only one to change it
	ps.prep_n = -1
	ps.v_a = ""
	ps.n_a = -1
	ps.accedMutex.Lock()
	ps.phase = None
	ps.numAcc = 0
	ps.numRej = 0
	ps.accedMutex.Unlock()

	return nil
}

// This function creates a commit message.
func (ps *paxosStates) CreateCommitMsg() ([]byte, error) {
	// no need to grab lock since we are the only thread reading mySeqNum
	// and v_a
	msg := &p_message{mtype: Commit, HostPort: ps.myHostPort, seqNum: ps.mySeqNum, val: ps.v_a}
	msgB, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, msgB)
	if err != nil {
		return nil, err
	}

	return generalMsg, nil
}

// This function is used by an acceptor to react to a commit
// message. It will react corresponding to the current state.
func (ps *paxosStates) receiveCommit(msg p_message) {
	val := msg.val
	// store the string into database
	//TODO Need to deal with duplicates
	tx, err := ps.storage.Begin()
	if err != nil {
		ps.logger.Printf("receiveCommit: database error. %s\n", err)
	} else {
		stmt, err := tx.Prepare("insert into storage(tweet, count) values(?, ?)")
		if err != nil {
			ps.logger.Printf("receiveCommit: database error. %s\n", err)
		}
		defer stmt.Close()
		if err != nil {
			_, err := stmt.Exec(val, 1)
			if err != nil {
				ps.logger.Printf("receiveCommit: exec error. %s\n", err)
			}
		}
		tx.Commit()
	}
}

// unmarshalls and determines the type of a p_message
// and calls the appropriate handler function
func (ps *paxosStates) Interpret_message(marshalled []byte) {
	var msg p_message
	json.Unmarshal(marshalled, &msg)

	switch msg.mtype {
	case None:
		ps.logger.Printf("Interpret_message: wrong message type\n")
	case Prepare:
		ps.logger.Printf("Interpret_message: received Prepare message\n")
		ps.receiveProposal(msg)
	case Accept:
		ps.logger.Printf("Interpret_message: received Accept message\n")
		//ps.receiveAcceptReq(msg)
	case PrepareReply:
		ps.logger.Printf("Interpret_message: received Prepare reply\n")
		ps.receivePrepareResponse(msg)
	case AcceptReply:
		ps.logger.Printf("Interpret_message: received Accept reply\n")
		ps.receiveAcceptResponse(msg)
	case Commit:
		ps.logger.Printf("Interpret_message: received Commit\n")
		ps.receiveCommit(msg)
	}
}

// This function sends a message to another server.
func (ps *paxosStates) sendMsg(port string, msgB []byte) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:"+port)
	if err != nil {
		ps.logger.Printf("sendMsg: error while connecting to server %s\n", port)
		return err
	} else {
		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			ps.logger.Printf("sendMsg: error while dialing to server %s\n", port)
			return err
		} else {
			_, err := conn.Write(msgB)
			if err != nil {
				ps.logger.Printf("sendMsg: error while writing to server %s\n", port)
				return err
			}
		}
	}
	return nil
}

// This function sends message to every other node in the network.
func (ps *paxosStates) broadCastMsg(msgB []byte) {
	for e := ps.nodes.Front(); e != nil; e = e.Next() {
		port := e.Value.(string)
		if port == ps.myHostPort {
			// skip sending message to itself
			continue
		}
		// send marshalled to ith node
		err := ps.sendMsg(port, msgB)
		if err != nil {
			ps.logger.Printf("Prepare: error while sending message. %s\n", err)
			// TODO what if the node is dead?
		}
	}
}
