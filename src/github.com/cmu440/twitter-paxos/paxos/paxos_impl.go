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
	"errors"
	_ "github.com/mattn/go-sqlite3"
    "math/rand"
	"sync"
    "time"

	"github.com/cmu440/twitter-paxos/message"
)

// Phase is either Prepare or Accept
type Phase int

const (
	None Phase = iota // Indicates the server is an acceptor
	Prepare
	Accept
	PrepareReply
	AcceptReply
	Commit
)

type TestSpec struct {
	// if rand.Float32() < rate { drop operation }
    // so 0 is perfect communication and 1 is no communication
	PingRate, prepSendRate, prepRespondRate, accSendRate, accRespondRate, commRate float32
	// <-time.After(time.Duration(del) * time.Millisecond) before operation
	// maybe add functionality for if del == -1 { wait random time }
	PingDel, prepSendDel, prepRespondDel, accSendDel, accRespondDel, commDel time.Duration
}

type paxosStates struct {
	// the following are constant once NewPaxos is called
	nodes          *list.List // list of all HostPorts
	myHostPort     string
	myID, numNodes int

	accedMutex     *sync.Mutex // protects the following variables
	numAcc, numRej int         // number of ok/reject response received
	phase          Phase       // which stage of paxos the server is at

	// TODO: use these and figure out locking
	iteration    int            // what this paxos thinks the iteration number is
	commitedVals map[int]string // commited values for each iteration
	accedVals    map[int]string // last acced value for each iteration

	accingMutex *sync.Mutex // protects the following variables
	n_h         int         // highest seqNum seen so far; 0 if none has seen
	n_a         int         // highest seqNum accepted; -1 if none
	v_a         string      // value associated with n_a
	prep_v      string      // latest value received during prepare stage
	prep_n      int         // largest proposal ID received during prepare stage
	mySeqNum    int         // seqNum of my proposal if proposing

	prepChan     chan bool          // channel for reporting quorum during prepare phase
	accChan      chan bool          // channel for reporting quorum during accept phase
	logger       *log.Logger        // logger for paxos (same as the one for server)
	msgHandler   message.MessageLib // message handler
	databaseFile string             // database used by storage server

	test TestSpec // parameters for testing
}

type p_message struct {
	Mtype    Phase
	HostPort string // for sending response
	SeqNum   int    // proposal ID corresponding to val
	Acc      bool   // True if reply OK
	Val      string // value related to the proposal. emptry string is there is no value to send
}

// This function creates a paxos package for storage server to use.
// nodes is an array of all host:port strings; this needs to be identical for
// each call to newPaxosStates (i.e., for each server), because it is used to
// generate the node's ID number, which in turn ensures that sequence numbers
// are unique to that node
func NewPaxosStates(myHostPort string, nodes *list.List,
	logger *log.Logger, databaseFile string, t TestSpec) PaxosStates {
	ps := &paxosStates{nodes: nodes, myHostPort: myHostPort, numNodes: nodes.Len(), databaseFile: databaseFile, test: t}
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
	ps.logger.Printf("Prepare: about to set phase\n")
	ps.accedMutex.Lock()
	if ps.phase != None {
		// server is busy handling the previous commit
		ps.accedMutex.Unlock()
		return false, nil
	}
	ps.phase = Prepare
	ps.accedMutex.Unlock()
	ps.logger.Printf("Prepare: finished setting phase.\n")

	// create prepare message
	ps.logger.Printf("Prepare: about to create prepare message\n")
	msgB, err := ps.CreatePrepareMsg()
	if err != nil {
		ps.logger.Printf("Prepare: error while creating prepare message. %s\n", err)
		return false, err
	}
	ps.logger.Printf("Prepare: finished creating prepare message\n")
	<-time.After(ps.test.prepSendDel)
	// send prepare message to the network
	ps.logger.Printf("Prepare: about to broadcast\n")
	ps.broadCastMsg(msgB, "prep")
	ps.logger.Printf("Prepare: finished broadcast\n")

	// wait for the majority to respond. Note that paxos doesn't
	// guarantee liveness. Therefore, we might wait forever due
	// to message loss.
	ps.logger.Printf("Prepare: wait for majority response\n")
	toReturn := false
	acc := true
	if ps.numNodes > 1 { // trivially accept if only one node
		acc = <-ps.prepChan
	}
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
	msg := &p_message{Mtype: Prepare, HostPort: ps.myHostPort, SeqNum: ps.n_h}
	ps.accingMutex.Unlock()
	ps.logger.Printf("CreatePrepareMsg: mtype = %d\n", msg.Mtype)
	ps.logger.Printf("CreatePrepareMsg: seqNum = %d\n", msg.SeqNum)
	msgB, err := json.Marshal(*msg)
	/* for debug */
	var t p_message
	json.Unmarshal(msgB, &t)
	ps.logger.Printf("CreatePrepareMsg: unmarshalled mtype = %d\n", t.Mtype)
	ps.logger.Printf("CreatePrepareMsg: unmarshalled seqNum = %d\n", t.SeqNum)
	if err != nil {
		ps.logger.Printf("CreatePrepareMsg: error while marshalling. %s\n", err)
		return nil, err
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, string(msgB))
	ps.logger.Printf("CreatePrepareMsg: msg string is %s\n", string(msgB))
	if err != nil {
		return nil, err
	}

	return generalMsg, nil
}

// This function is used by an acceptor to react to a prepare
// message. It will react corresponding to the current state.
func (ps *paxosStates) receiveProposal(msg p_message) {
	resp := &p_message{Mtype: PrepareReply, HostPort: ps.myHostPort}
	ps.accedMutex.Lock()
	if ps.phase == None {
		// this server is acceptor
		ps.accingMutex.Lock()
		if msg.SeqNum > ps.n_h { // accept
			resp.Acc = true
			resp.Val = ps.v_a
			resp.SeqNum = ps.n_a
			ps.n_h = msg.SeqNum
		} else { // reject
			resp.Acc = false
		}
		ps.accingMutex.Unlock()
	} else {
		// this server is leader. Prevent two leaders in one system
		// leader must reject a prepare request of another node
		resp.Acc = false
	}
	ps.accedMutex.Unlock()

	// send out response message
	if rand.Float32() < ps.test.prepRespondRate {
		ps.logger.Printf("receiveProposal: dropped prepare response message.\n")
		return
	}
	<-time.After(ps.test.prepRespondDel)
	msgB, err := json.Marshal(resp)
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, string(msgB))
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
	ps.logger.Printf("receivePrepareResponse: enter function\n")
	ps.accedMutex.Lock()
	ps.logger.Printf("receivePrepareResponse: get lock\n")
	if ps.phase == Prepare {
		if msg.Acc {
			ps.numAcc++
			// udpate the uncommitted value and corresponding proposal ID
			if msg.Val != "" {
				ps.accingMutex.Lock()
				if msg.SeqNum > ps.prep_n {
					ps.prep_n = msg.SeqNum
					ps.prep_v = msg.Val
				}
				ps.accingMutex.Unlock()
			}
		} else {
			ps.numRej++
		}

		ps.logger.Printf("receivedPrepareResponse: numAcc is %d\n", ps.numAcc)
		if 2*ps.numAcc > ps.numNodes { // proposal accepted
			ps.prepChan <- true
		} else if 2*ps.numRej > ps.numNodes { // proposal rejected
			ps.prepChan <- false
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

	<-time.After(ps.test.accSendDel)
	// send accept message to the network
	ps.broadCastMsg(msgB, "acc")

	// wait for the majority to respond. Note that paxos doesn't
	// guarantee liveness. Therefore, we might wait forever due
	// to message loss.
	toReturn := false
	acc := true
	if ps.numNodes > 1 {
		acc = <-ps.accChan
	}
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
	msg := &p_message{Mtype: Accept, HostPort: ps.myHostPort, SeqNum: ps.mySeqNum, Val: val}
	msgB, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, string(msgB))
	if err != nil {
		return nil, err
	}

	return generalMsg, nil
}

// This function is used by an acceptor to react to an accept request.
// It will react corresponding to the current state.
func (ps *paxosStates) receiveAccept(msg p_message) {
	resp := &p_message{Mtype: AcceptReply, HostPort: ps.myHostPort, SeqNum: msg.SeqNum}
	ps.accedMutex.Lock()
	if ps.phase == None {
		// this server is acceptor
		ps.accingMutex.Lock()
		if msg.SeqNum >= ps.n_h { // accept
			resp.Acc = true
			// update v_a and n_a
			ps.v_a = msg.Val
			ps.n_a = msg.SeqNum
		} else { // reject
			resp.Acc = false
		}
		ps.accingMutex.Unlock()
	} else {
		// this server is leader. Prevent two leaders in one system
		// leader must reject an accept request of another node
		resp.Acc = false
	}
	ps.accedMutex.Unlock()

	// send out response message
	if rand.Float32() < ps.test.accRespondRate {
		ps.logger.Printf("receiveProposal: dropped accept response message.\n")
		return
	}
	<-time.After(ps.test.accRespondDel)
	msgB, err := json.Marshal(resp)
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, string(msgB))
	if err != nil {
		ps.logger.Printf("receiveAccept: error creating message. %s\n", err)
	} else {
		err = ps.sendMsg(msg.HostPort, generalMsg)
		if err != nil {
			ps.logger.Printf("receiveAccept: error sending message. %s\n", err)
		}
	}
}

// This function is used by a leader upon receiving an accept
// response from an acceptor. It will react corresponding to
// the message and make updates to the current state.
func (ps *paxosStates) receiveAcceptResponse(msg p_message) {
	ps.accedMutex.Lock()
	if ps.phase == Accept {
		if msg.SeqNum == ps.mySeqNum { // ignore old responses
			if msg.Acc {
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
func (ps *paxosStates) commitVal() (string, error) {
	// construct commit message
	msgB, err := ps.CreateCommitMsg()
	if err != nil {
		return "", err
	}

	// send prepare message to the network
	time.After(ps.test.commDel)
	ps.broadCastMsg(msgB, "comm")

	// commit on its local machine. since this is the only thead
	// reading v_a, therefore no need to grab the lock.
	val := ps.v_a
	db, err := sql.Open("sqlite3", ps.databaseFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	query := "insert into storage(tweet, count) values ('" + val + "'"
	query += ", 1)"
	_, err = db.Exec(query)
	if err != nil {
		// TODO should we return error here?
		ps.logger.Printf("receiveCommit: error while executing query. %s\n", err)
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

	return val, nil
}

// This function creates a commit message.
func (ps *paxosStates) CreateCommitMsg() ([]byte, error) {
	// no need to grab lock since we are the only thread reading mySeqNum
	// and v_a
	msg := &p_message{Mtype: Commit, HostPort: ps.myHostPort, SeqNum: ps.mySeqNum, Val: ps.v_a}
	msgB, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, string(msgB))
	if err != nil {
		return nil, err
	}

	return generalMsg, nil
}

// This function is used by an acceptor to react to a commit
// message. It will react corresponding to the current state.
func (ps *paxosStates) receiveCommit(msg p_message) {
	val := msg.Val
	ps.logger.Printf("receiveCommit: value to be commited is %s\n", val)
	// store the string into database
	//TODO Need to deal with duplicates
	db, err := sql.Open("sqlite3", ps.databaseFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	query := "insert into storage(tweet, count) values ('" + val + "'"
	query += ", 1)"
	_, err = db.Exec(query)
	if err != nil {
		ps.logger.Printf("receiveCommit: error while executing query. %s\n", err)
	}
}

// This function does a commit. It is the only access point for server
// to communicate with paxos algorithm.
// This function will return the value committed and error message if any.
// When error occurs, this function will return an empty string along
// with an error message.
//
//TODO We need to think more about returning error in this function.
//     Returning error must gurantee that the value won't be committed
//     in the future without retrying.
func (ps *paxosStates) PaxosCommit(val string) (string, error) {
	ps.logger.Printf("enter PaxosCommit\n")
	prepSuccess, err := ps.Prepare()
	ps.logger.Printf("PaxosCommit: finished prepare phase\n")
	if err != nil {
		return "", err
	}
	if prepSuccess {
		// Success in Prepare phase. Move on to Accept phase
		accSuccess, err := ps.Accept(val)
		if err != nil {
			return "", err
		}
		if accSuccess {
			// Success in Accept phase. Move on to Commit phase
			val, err := ps.commitVal()
			if err != nil {
				// commit failed
				return val, err
			} else {
				// commit success
				return val, nil
			}
		} else {
			// Failure in Accept phase.
			return "", nil
		}
	} else {
		// Failure in Prepare phase.
		// TODO for now, we will just return error since this value
		//      will never be commited as the server isn't elected
		//      as a leader.
		return "", errors.New("not commit")
	}
}

// unmarshalls and determines the type of a p_message
// and calls the appropriate handler function
func (ps *paxosStates) Interpret_message(marshalled []byte) {
	ps.logger.Printf("Interpret_message: enter function\n")
	var msg p_message
	err := json.Unmarshal(marshalled, &msg)
	if err != nil {
		ps.logger.Printf("Interpret_message: unmarshal error. %s\n", err)
	}

	switch msg.Mtype {
	case None:
		ps.logger.Printf("Interpret_message: wrong message type\n")
	case Prepare:
		ps.logger.Printf("Interpret_message: received Prepare message\n")
		ps.receiveProposal(msg)
	case Accept:
		ps.logger.Printf("Interpret_message: received Accept message\n")
		ps.receiveAccept(msg)
	case PrepareReply:
		ps.logger.Printf("Interpret_message: received Prepare reply\n")
		ps.receivePrepareResponse(msg)
	case AcceptReply:
		ps.logger.Printf("Interpret_message: received Accept reply\n")
		ps.receiveAcceptResponse(msg)
	case Commit:
		ps.logger.Printf("Interpret_message: received Commit\n")
		ps.receiveCommit(msg)
	default:
		ps.logger.Printf("fuck\n")
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
	ps.logger.Printf("sendMsg: finished sending message to %s\n", port)
	return nil
}

// This function sends message to every other node in the network.
// mType is just for determining the drop rate for testing
func (ps *paxosStates) broadCastMsg(msgB []byte, mType string) {
	for e := ps.nodes.Front(); e != nil; e = e.Next() {
		port := e.Value.(string)
		if port == ps.myHostPort {
			// skip sending message to itself
			continue
		}

		// drop messages for testing
		if mType == "prep" && rand.Float32() < ps.test.prepSendRate {
			ps.logger.Printf("dropped prepare message. %s\n")
			continue
		}
		if mType == "acc" && rand.Float32() < ps.test.accSendRate {
			ps.logger.Printf("dropped accept message. %s\n")
			continue
		}
		if mType == "comm" && rand.Float32() < ps.test.commRate {
			ps.logger.Printf("dropped commit message. %s\n")
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
