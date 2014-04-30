// Implementation for paxos
package paxos

import (
	"encoding/json"
	"fmt"
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
	"os"
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
	NOOP
)

type TestSpec struct {
	// if rand.Float32() < rate { drop operation }
	// so 0 is perfect communication and 1 is no communication
	PingRate, PrepSendRate, PrepRespondRate, AccSendRate, AccRespondRate, CommRate float32
	// <-time.After(time.Duration(del) * time.Millisecond) before operation
	// maybe add functionality for if del == -1 { wait random time }
	PingDel, PrepSendDel, PrepRespondDel, AccSendDel, AccRespondDel, CommDel time.Duration

	StartTime time.Duration

	Ignore []string // list of HostPorts to ignore messages from

	DontRegister bool // whether or not to call rpc.RegisterName (only call once)

	Testing bool // true iff testing
	DropAll bool // true iff currently dropping all messages
}

type paxosStates struct {
	// the following are constant once NewPaxos is called
	nodes          *list.List // list of all HostPorts
	myHostPort     string
	myID, numNodes int

	accedMutex     *sync.Mutex    // protects the following variables
	numAcc, numRej int            // number of ok/reject response received
	phase          Phase          // which stage of paxos the server is at
	iteration      int            // what this paxos thinks the iteration number is
	commitedVals   map[int]string // commited values for each iteration
	behind         bool           // True means the node's iteration is behind
	//accedVals      map[int]string // last acced value for each iteration TODO why do we need this?

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
	commitLogger *log.Logger        // logger for commits
	msgHandler   message.MessageLib // message handler
	databaseFile string             // database used by storage server

	test TestSpec // parameters for testing
}

type p_message struct {
	Mtype     Phase
	HostPort  string // for sending response
	SeqNum    int    // proposal ID corresponding to val
	Iteration int    // iteration number
	Acc       bool   // True if reply OK
	Val       string // value related to the proposal. emptry string is there is no value to send
}

// This function creates a paxos package for storage server to use.
// nodes is an array of all host:port strings; this needs to be identical for
// each call to newPaxosStates (i.e., for each server), because it is used to
// generate the node's ID number, which in turn ensures that sequence numbers
// are unique to that node
func NewPaxosStates(myHostPort string, nodes *list.List,
	logger *log.Logger, databaseFile string, t TestSpec) PaxosStates {
	fmt.Printf("Number of nodes: %d\n", nodes.Len())
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
	ps.iteration = 0
	ps.behind = false
	ps.commitedVals = make(map[int]string)
	ps.n_h = 0
	ps.n_a = -1
	ps.v_a = ""
	ps.prep_v = ""
	ps.prep_n = -1
	ps.prepChan = make(chan bool)
	ps.accChan = make(chan bool)
	ps.logger = logger
	ps.msgHandler = message.NewMessageHandler()

	// create commit logger
	filename := "commit" + ps.myHostPort + ".txt"
	logFile, _ := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	ps.commitLogger = log.New(logFile, "", log.Lshortfile)

	// start server late (i.e., drop all messages before StartTime)
	if int64(ps.test.StartTime) > 0 {
		ps.test.DropAll = true
		go func() {
			<-time.After(ps.test.StartTime)
			ps.test.DropAll = false
		}()

	}

	return ps
}

// This function is used by a node to make a new proposal
// to other nodes. It will return true when a proposal is
// being successfully made. Otherwise, it will return false.
func (ps *paxosStates) Prepare() (bool, error) {
	// set phase and extract iteration number
	ps.logger.Printf("Prepare: enter function\n")
	var iter int
	ps.accedMutex.Lock()
	ps.numAcc = 1
	if ps.phase != None {
		// server is busy handling the previous commit
		ps.accedMutex.Unlock()
		return false, nil
	}
	ps.phase = Prepare
	iter = ps.iteration
	ps.numAcc += 1
	ps.accedMutex.Unlock()

	// create prepare message
	msgB, err := ps.CreatePrepareMsg(iter)
	if err != nil {
		ps.logger.Printf("Prepare: error while creating prepare message. %s\n", err)
		return false, err
	}
	<-time.After(ps.test.PrepSendDel)
	// send prepare message to the network
	ps.broadCastMsg(msgB, "prep")

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
	} else {
		ps.logger.Printf("Prepare: prepare-reject from majority\n")
	}

	return toReturn, nil
}

// This function returns a marshalled Prepare p_message with
// (hopefully) a highest new seqNum
// iter Current iteration the node is on
func (ps *paxosStates) CreatePrepareMsg(iter int) ([]byte, error) {
	// generate a sequence number, ensured to be unique by
	// taking next multiple of numNodes and adding myID
	ps.accingMutex.Lock()
	ps.n_h = ps.numNodes*(1+(ps.n_h/ps.numNodes)) + ps.myID
	ps.mySeqNum = ps.n_h
	msg := &p_message{
		Mtype:     Prepare,
		HostPort:  ps.myHostPort,
		SeqNum:    ps.n_h,
		Iteration: iter}
	ps.accingMutex.Unlock()
	msgB, err := json.Marshal(*msg)
	if err != nil {
		ps.logger.Printf("CreatePrepareMsg: error while marshalling. %s\n", err)
		return nil, err
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, string(msgB))
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
		if ps.iteration == msg.Iteration {
			// acceptor is on the same iteration as the proposing node
			// start normal paxos algorithm
			ps.accingMutex.Lock()
			if msg.SeqNum > ps.n_h { // accept
				ps.logger.Printf("receiveProposal: accept proposal from server %s with v.a set to %s and n_a be %d at iteration %d\n", msg.HostPort, ps.v_a, ps.n_a, ps.iteration)
				resp.Acc = true
				resp.Val = ps.v_a
				resp.SeqNum = ps.n_a
				resp.Iteration = msg.Iteration
				ps.n_h = msg.SeqNum
			} else { // reject
				ps.logger.Printf("receiveProposal: reject proposal from server %s\n", msg.HostPort)
				resp.Acc = false
				resp.Iteration = -1
			}
			ps.accingMutex.Unlock()
		} else if ps.iteration < msg.Iteration {
			// acceptor is behind the proposing node. reject proposal
			ps.logger.Printf("receiveProposal: shouldn't happen now. my iteration is %d while proposing node is at iteration %d\n", ps.iteration, msg.Iteration)
			resp.Acc = false
			resp.Iteration = ps.iteration
			// TODO trigger no-op recovery process
		} else {
			// acceptor is beyond the proposing node
			// reject proposal
			resp.Acc = false
			ps.logger.Printf("receiveProposal: shouldn't happen now. my iteration is %d while proposing node is at iteration %d\n", ps.iteration, msg.Iteration)
			// retrieve committed value for that iteration and
			// send to proposing node
			resp.Val = ps.commitedVals[msg.Iteration]
			resp.SeqNum = -1
			resp.Iteration = msg.Iteration
		}
	} else {
		// this server is leader. Prevent two leaders in one system
		// leader must reject a prepare request of another node
		ps.logger.Printf("receiveProposal: reject proposal from server %s since I am leader.\n", msg.HostPort)
		resp.Acc = false
		resp.Iteration = -1
	}
	ps.accedMutex.Unlock()

	if ps.test.DropAll || rand.Float32() < ps.test.PrepRespondRate {
		ps.logger.Printf("receiveProposal: dropped prepare response message.\n")
		return
	}
	<-time.After(ps.test.PrepRespondDel)

	// send out response message
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
	ps.accedMutex.Lock()
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
			// figure out reason for rejecting
			if msg.Iteration != -1 {
				// rejected due to unmatching iteration number
				if msg.Iteration == ps.iteration {
					// we are behind.
					// TODO should we start no-op recovery after
					//      a majority says we are behind?
					ps.behind = true
					// TODO do we need to commit the value returned by
					//      the node?
					ps.commitedVals[ps.iteration] = msg.Val
				}
			}
		}

		if 2*ps.numAcc > ps.numNodes { // proposal accepted
			ps.prepChan <- true
			ps.phase = Accept // prevent extra prepare-ok message from blocking the algorithm since the channel is not buffered
			ps.numAcc = 1     // reset numAcc
			ps.numRej = 0     // reset numRej
		} else if 2*ps.numRej > ps.numNodes { // proposal rejected
			ps.prepChan <- false
			ps.phase = None
		}
	}

	ps.accedMutex.Unlock()
}

// This function is used by a leader to do all the work in
// accept phase. It will return true when a majority of nodes
// reply accept-ok. Otherwise, it will return false.
func (ps *paxosStates) Accept(val string) (bool, error) {
	ps.logger.Printf("Accept: enter function.\n")

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
	ps.logger.Printf("Accept: value to be proposed is %s\n", accept_val)

	// create accept message
	msgB, err := ps.CreateAcceptMsg(accept_val)
	if err != nil {
		ps.logger.Printf("Accept: error while creating accept message. %s\n", err)
		return false, err
	}

	<-time.After(ps.test.AccSendDel)
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
		ps.logger.Printf("Accept: accept-ok from majority\n")
		toReturn = true
		// update v_a and n_a. Since we are the only thread updating,
		// no need to grab lock
		ps.v_a = accept_val
		ps.n_a = ps.mySeqNum
	} else {
		// majority rejects. reset paxos state
		ps.logger.Printf("Accept: accept-reject from majority\n")
		ps.prep_v = ""
		ps.prep_n = -1
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
	if ps.test.DropAll || rand.Float32() < ps.test.AccRespondRate {
		ps.logger.Printf("receiveProposal: dropped accept response message.\n")
		return
	}
	<-time.After(ps.test.AccRespondDel)

	// send out response message
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
				ps.phase = Commit // prevent extra accept response from blocking paxos algorithm since the channel is unbuffered
			} else if 2*ps.numRej > ps.numNodes { // majority rejects
				ps.accChan <- false
				ps.phase = None
				ps.numAcc = 0
				ps.numRej = 0
			}
		}
	}

	ps.accedMutex.Unlock()
}

// This function is used by a leader to commit a value after
// a successful accept phase.
func (ps *paxosStates) commitVal() (string, error) {
	ps.logger.Printf("commitVal: enter function\n")
	// construct commit message
	msgB, err := ps.CreateCommitMsg()
	if err != nil {
		return "", err
	}

	// send prepare message to the network
	time.After(ps.test.CommDel)
	ps.broadCastMsg(msgB, "comm")

	// commit on its local machine. since this is the only thead
	// reading v_a, therefore no need to grab the lock.
	ps.logger.Printf("commitVal: begin to store val into database.\n")
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

	/* log commit */
	ps.logger.Printf("commitVal: commit %s at iteration %d\n", val, ps.iteration)
	ps.commitLogger.Printf("commit:" + val)

	// reset phase variable to None
	ps.prep_v = "" // no need to grab lock since we are the only one to change it
	ps.prep_n = -1
	ps.v_a = ""
	ps.n_a = -1
	ps.accedMutex.Lock()
	ps.phase = None
	ps.numAcc = 0
	ps.numRej = 0
	ps.commitedVals[ps.iteration] = val
	ps.iteration += 1
	ps.accedMutex.Unlock()
	ps.logger.Printf("commitVal: finished updating iteration number.\n")

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
	// grab lock here to prevent Prepare gets called before
	// iteration number gets updated. Prevent Prepare from
	// using wrong iteration number.
	ps.accedMutex.Lock()
	ps.accingMutex.Lock()
	val := msg.Val
	ps.logger.Printf("receiveCommit: value to be commited is %s at iteration %d\n", val, ps.iteration)

	/* log the commit */
	ps.commitLogger.Printf("commit:" + val)

	/* udpate iteration count */
	ps.commitedVals[ps.iteration] = val
	ps.iteration += 1

	ps.logger.Printf("receiveCommit: finished upding iteration count.\n")

	/* reset state */
	ps.n_a = -1
	ps.v_a = ""
	ps.accingMutex.Unlock()
	ps.accedMutex.Unlock()
	ps.logger.Printf("receiveCommit: finished resetting state.\n")

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
	prepSuccess, err := ps.Prepare()
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

		// TODO check whether the node is behind. If so, run no-op recovery
		return "", errors.New("not commit")
	}
}

// This go rountine will do no-op recovery.
// When a node realizes it is behind, it will send no-op messages along
// with its iteration numbers to all other nodes. It will start with
// iteration number 0 and gradually go up. Once it receives a response
// for a iteration number, it should commit that value. The recovery
// should stop when a majority of nodes return no value for a iteration
// number.
func (ps *paxosStates) noopRecovery() {
}

// This function creates a noop message for a particular iteration
// number.
func (ps *paxosStates) CreateNoopMsg(iter int) ([]byte, error) {
	msg := &p_message{
		Mtype:     NOOP,
		HostPort:  ps.myHostPort,
		Iteration: iter}
	msgB, err := json.Marshal(*msg)
	if err != nil {
		ps.logger.Printf("CreateNoopMsg: error while marshalling. %s\n", err)
	}
	generalMsg, err := ps.msgHandler.CreateMsg(message.PAXOS, string(msgB))
	if err != nil {
		return nil, err
	}

	return generalMsg, nil
}

// unmarshalls and determines the type of a p_message
// and calls the appropriate handler function
func (ps *paxosStates) Interpret_message(marshalled []byte) {
	var msg p_message
	err := json.Unmarshal(marshalled, &msg)
	if err != nil {
		ps.logger.Printf("Interpret_message: unmarshal error. %s\n", err)
	}

	// fmt.Printf("Server %s receiving message from %s\n", ps.myHostPort, msg.HostPort)

	for i := 0; i < len(ps.test.Ignore); i++ {
		if msg.HostPort == ps.test.Ignore[i] {
			// fmt.Printf("Server %s ignoring message from %s\n", ps.myHostPort, msg.HostPort)
			return // ignore message
		}
	}

	switch msg.Mtype {
	case None:
		ps.logger.Printf("Interpret_message: wrong message type from %s\n", msg.HostPort)
	case Prepare:
		ps.logger.Printf("Interpret_message: received Prepare message from %s\n", msg.HostPort)
		ps.receiveProposal(msg)
	case Accept:
		ps.logger.Printf("Interpret_message: received Accept message from %s\n", msg.HostPort)
		ps.receiveAccept(msg)
	case PrepareReply:
		ps.logger.Printf("Interpret_message: received Prepare reply from %s\n", msg.HostPort)
		ps.receivePrepareResponse(msg)
	case AcceptReply:
		ps.logger.Printf("Interpret_message: received Accept reply from %s\n", msg.HostPort)
		ps.receiveAcceptResponse(msg)
	case Commit:
		ps.logger.Printf("Interpret_message: received Commit from %s\n", msg.HostPort)
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

		if ps.test.DropAll {
			ps.logger.Printf("dropped %s message.\n", mType)
			continue
		}

		// drop messages for testing
		if mType == "prep" && rand.Float32() < ps.test.PrepSendRate {
			ps.logger.Printf("dropped prepare message.\n")
			continue
		}
		if mType == "acc" && rand.Float32() < ps.test.AccSendRate {
			ps.logger.Printf("dropped accept message.\n")
			continue
		}
		if mType == "comm" && rand.Float32() < ps.test.CommRate {
			ps.logger.Printf("dropped commit message.\n")
			continue
		}

		// send marshalled to ith node
		err := ps.sendMsg(port, msgB)
		if err != nil {
			ps.logger.Printf("broadcast: error while sending message. %s\n", err)
			// TODO what if the node is dead?
		}
	}
}
