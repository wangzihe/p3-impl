// Implementation of storage server
package storageserver

import (
	"bufio"
	"container/list"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"

	"github.com/cmu440/twitter-paxos/message"
	"github.com/cmu440/twitter-paxos/paxos"
	"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

type Status int

// Message types
const (
	serverBufSize int    = 1500 // buffer size for reading server message
	TIMEOUT       int64  = 10   // time out for ping
	RETRY         int    = 5    // number of times to retry during ping
	RPCPORT       int    = 0    // configuration file type (contains port for rpc)
	MSGPORT       int    = 1    // configuration file type. (contains port for messaging)
	PING_ASK      Status = iota + 1
	PING_REPLY
)

type storageServer struct {
	PortRPC        string             // port string for RPC
	PortMsg        string             // port string for message passing
	Config         string             // path to the configuration file
	ServerRPCPorts *list.List         // list of storage server rpc port strings
	ServerMsgPorts *list.List         // list of storage server message port strings
	RPCListener    net.Listener       // listen socket for rpc
	MsgListener    net.Listener       // listen socket for messages
	pingChan       chan string        // channel for ping messages between network handler and server
	LOGV           *log.Logger        // server logger
	MsgHandler     message.MessageLib // message handler
	PaxosHandler   paxos.PaxosStates  // paxos handler
	test           paxos.TestSpec     // message drop-rates/delays for testing
}

// Server message. Servers will send marshalled string of
// messages around. Each marshalled string of messages is
// followed by a newline to indicate the end of message.
type serverMsg struct {
	MsgType    Status // type of message
	Value      string // content of message. Value for Paxos algorithm. Empty string for ping messages.
	ProposalID string // proposal ID. Empty string for ping messages.
}

// This function parses the configuration file that contains all port
// string for all the storage servers in the network and store those string
// into a list.
func (ss *storageServer) parseConfigFile(config string,
	configType int) error {
	f, err := os.Open(config)
	if err != nil {
		ss.LOGV.Printf("parseConfigFile: error while opening file %s. %s\n", ss.Config, err)
		return err
	}

	reader := bufio.NewReader(f)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if configType == RPCPORT {
			ss.ServerRPCPorts.PushBack(scanner.Text())
		} else {
			ss.ServerMsgPorts.PushBack(scanner.Text())
		}
	}
	if err = scanner.Err(); err != nil {
		ss.LOGV.Printf("parseConfigFile: scanner error. %s\n", err)
		return err
	}

	return nil
}

// This function read a high-level message from the connection.
// This message can embed a server message or a paxos message.
// The function will return the byte array for the message embeded
// inside the high-level message, message type and error
func (ss *storageServer) readMsg(conn net.Conn) ([]byte, int, error) {
	ss.LOGV.Printf("readMsg: enter function\n")
	reader := bufio.NewReader(conn)
	msgBytes, err := reader.ReadBytes('\n')
	ss.LOGV.Printf("readMsg: finished retrieving bytes from network\n")
	if err != nil {
		// error occurred while reading server message
		fmt.Printf("readMsg: error while reading server message. %s\n", err)
		return nil, -1, err
	}
	generalMsgB, msgType, err := ss.MsgHandler.RetrieveMsg(msgBytes)
	ss.LOGV.Printf("readMsg: finished retriving general message\n")
	if err != nil {
		return nil, -1, err
	} else {
		return generalMsgB, msgType, nil
	}
}

// This function is used by server to parse server message. It will
// return a pointer to the server message struct.
func (ss *storageServer) parseServerMsg(msgB []byte) *serverMsg {
	var msg serverMsg
	err := json.Unmarshal(msgB, &msg)
	if err != nil {
		fmt.Printf("readMsg: error while unmarshalling. %s\n", err)
		return nil
	} else {
		return &msg
	}
}

// This function constructs a server message.
func makeMsg(msgType Status, value string, proposalID string) *serverMsg {
	return &serverMsg{
		MsgType:    msgType,
		Value:      value,
		ProposalID: proposalID,
	}
}

// This function converts a server message struct to a string ended with
// "\n" and then convert that string into a series of bytes. This allows
// us to send the marshalled server message through network.
func wrapMsg(msg *serverMsg) ([]byte, error) {
	msgB, err := json.Marshal(*msg)
	if err != nil {
		return nil, err
	}
	temp := string(msgB) + "\n"
	return []byte(temp), nil
}

// This is the go routine that will get ping response from a particular
// server.
// sigChan Channel to send signal back to main thread
// conn    Client connection that the particular server
/*
func (ss *storageServer) getPingResponse(sigChan chan struct{},
	conn net.Conn) {
	msgType, _, _, err := readMsg(conn)
	if err != nil {
		ss.LOGV.Printf("getPingResponse: error while reading server message. %s\n", err)
		conn.Close()
	}
	if msgType == PING_REPLY {
		ss.LOGV.Printf("getPingResponse: received a ping response.\n")
		sigChan <- struct{}{}
		conn.Close()
		return
	}
}*/

// This go routine implements ping timeout. After TIME_OUT seconds,
// it will send a signal to main thread through a timeout channel.
/*
func (ss *storageServer) pingTimeout(timeoutChan chan struct{}) {
	time.Sleep(time.Duration(TIMEOUT) * time.Second)
	timeoutChan <- struct{}{}
	ss.LOGV.Printf("pingTimeout: terminate\n")
}
*/

// This function pings a particular server. It inputs a port number for the
// particular server it wants to ping. If there is any error occurred or a
// timeout happens, it will return false. Otherwise, it will return true.
/*
func (ss *storageServer) ping(port string) bool {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:"+port)
	if err != nil {
		ss.LOGV.Printf("ping: error occurred while pinging server %s.%s\n", port, err)
		// sleep for TIMEOUT seconds before returning. Give some time
		// before another retry
		time.Sleep(time.Duration(TIMEOUT) * time.Second)
		return false
	} else {
		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			ss.LOGV.Printf("ping: error occurred while contacting server %s. %s\n", port, err)
			// sleep for TIMEOUT seconds before returning. Give some time
			// before another retry
			time.Sleep(time.Duration(TIMEOUT) * time.Second)
			return false
		}
		msg := makeMsg(PING_ASK, "", "")
		msgB, _ := wrapMsg(msg)
		_, err = conn.Write(msgB)
		if err != nil {
			ss.LOGV.Printf("ping: error occurred while messaging server %s. %s\n", port, err)
			conn.Close()
			return false
		}
		// create go routine to accept ping response and a go routine
		// for timeout
		sigChan := make(chan struct{})
		timeoutChan := make(chan struct{})
		go ss.getPingResponse(sigChan, conn)
		go ss.pingTimeout(timeoutChan)
		select {
		case <-sigChan:
			// received ping response
			conn.Close()
			return true
		case <-timeoutChan:
			// received time out signal
			conn.Close()
			return false
		}
	}
}
*/

// This function sends ping messages to all other storage servers
// in the network to make sure they have all successfully started.
// For each storage server, it will try up to 5 times before it
// gives up and returns error. The function will return true when
// all servers are running. False otherwise.
/*
func (ss *storageServer) pingServers() bool {
	for e := ss.ServerPorts.Front(); e != nil; e = e.Next() {
		port := e.Value.(string)
		if port == ss.Hostport {
			continue
		}
		ss.LOGV.Printf("ping server %s\n", port)
		var fail bool = true
		for index := 0; index < RETRY; index++ {
			ss.LOGV.Printf("ping\n")
			success := ss.ping(port)
			if success == true {
				fail = false
				break
			}
		}
		if fail == true {
			return false
		}
	}

	return true
}
*/

// This is the handler to handle messages received by the server.
func (ss *storageServer) networkHandler() {
	ss.LOGV.Printf("networkHandler: start listening\n")
	listener := ss.MsgListener

	for {
		conn, err := listener.Accept()
		if err != nil {
			// listen socket is closed by the main thread
			ss.LOGV.Printf("networkHandler: error while accpeting. %s\n", err)
			return
		} else {
			// read server message
			ss.LOGV.Printf("networkHandler: received a message\n")
			msgB, msgType, errR := ss.readMsg(conn)

			if errR != nil {
				ss.LOGV.Printf("networkHandler: error while reading msg. %s\n", errR)
			} else {
				switch msgType {
				case message.SERVER:
					// received a server message
					ss.parseServerMsg(msgB)
					conn.Close()
				case message.PAXOS:
					// received a paxos message
					ss.LOGV.Printf("networkHandler: received a paxos message\n")
					go ss.PaxosHandler.Interpret_message(msgB)
					conn.Close()
				}
			}
		}
	}
}

// This function sends ping messages to all other storage servers
// in the network to make sure they have all successfully started.
// For each storage server, it will try up to 5 times before it
// gives up and returns error. The function will return true when
// all servers are running. False otherwise.
func (ss *storageServer) pingServers() bool {
	for e := ss.ServerRPCPorts.Front(); e != nil; e = e.Next() {
		port := e.Value.(string)
		if port == ss.PortRPC {
			continue
		}
		ss.LOGV.Printf("ping server %s\n", port)
		var fail bool = true
		for index := 0; index < RETRY; index++ {
			if rand.Float32() < ss.test.PingRate {
				ss.LOGV.Printf("pingServers: dropped ping message\n")
				continue
			}
			ss.LOGV.Printf("ping\n")
			cli, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", port))
			if err != nil {
				ss.LOGV.Printf("error dialing tcp. %s\n", err)
				time.Sleep(time.Duration(TIMEOUT) * time.Second)
				continue
			} else {
				var a int = 1
				var b int = 2
				err = cli.Call("StorageServer.Ping", &a, &b)
				if err != nil {
					ss.LOGV.Printf("error calling commit. %s\n", err)
					continue
				} else {
					fail = false
					break
				}
			}
		}
		if fail == true {
			return false
		}
	}
	return true
}

// This function creates a new storage server.
// portRPC: RPC port string of the storage server
// portMsg: message port string of the storage server
// configRPC: path to the configuration file for the server. The
//            configuration file contains the list of rpc ports
//            of storage servers.
// configMsg: path to the configuration file for the server. The
//            configuration file contains the list of message ports
//            of storage servers.
func NewStorageServer(portRPC, portMsg, configRPC, configMsg string, test paxos.TestSpec) (StorageServer, error) {
	fmt.Printf("new storage server created\n")
	server := new(storageServer)
	server.PortRPC = portRPC
	server.PortMsg = portMsg
	server.ServerRPCPorts = list.New()
	server.ServerMsgPorts = list.New()
	server.MsgHandler = message.NewMessageHandler()
	server.test = test

	// create database file
	databaseFile := "./storage" + portMsg + ".db"
	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	sql := `
		create table if not exists storage (tweet text not null primary key, count int);
		`
	_, err = db.Exec(sql)
	if err != nil {
		log.Printf("%q: %s\n", err, sql)
		return nil, err
	}

	// create log file for server
	filename := portRPC + ".txt"
	logFile, _ := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	server.LOGV = log.New(logFile, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	server.LOGV.Printf("starting storage server on RPC port %s and msg port %s\n", portRPC, portMsg)

	// parse configuration file
	server.parseConfigFile(configRPC, RPCPORT)
	server.parseConfigFile(configMsg, MSGPORT)
	// set up listen sockets
	RPCListenPort := ":" + portRPC
	rpcLn, err := net.Listen("tcp", RPCListenPort)
	if err != nil {
		server.LOGV.Printf("NewStorageServer: error while creating listen socket for rpc. %s\n", err)
		return nil, err
	}
	server.RPCListener = rpcLn
	msgListenPort := ":" + portMsg
	server.LOGV.Printf("msgListener is on %s\n", portMsg)
	msgLn, err := net.Listen("tcp", msgListenPort)
	if err != nil {
		server.LOGV.Printf("NewStorageServer: error while creating message listen socket.\n", err)
		return nil, err
	}
	server.MsgListener = msgLn

	if !test.DontRegister {
		// Wrap the storageserver before registering it for RPC
		err = rpc.RegisterName("StorageServer", storagerpc.Wrap(server))
		if err != nil {
			server.LOGV.Printf("NewStorageServer: error while registering rpc. %s\n", err)
			return nil, err
		}
		rpc.HandleHTTP()
	} else if test.Testing {
		<-time.After(time.Second)
	}
	go http.Serve(rpcLn, nil)

	go server.networkHandler()
	// ping all other storage servers
	/*

		if server.pingServers() == false {
			server.LOGV.Printf("some servers failed to start\n")
			server.Ln.Close()
			return nil, errors.New("not all servers exist")
		}

		fmt.Printf("finished ping\n")
	*/
	if server.pingServers() == false {
		server.LOGV.Printf("some servers failed to start\n")
		server.RPCListener.Close()
		server.MsgListener.Close()
		return nil, errors.New("not all servers exist")
	}
	server.PaxosHandler = paxos.NewPaxosStates(portMsg,
		server.ServerMsgPorts, server.LOGV, databaseFile, test)

	return server, nil
}

func (ss *storageServer) Ping(a, b *int) error {
	<-time.After(ss.test.PingDel)
	fmt.Printf("ping called\n")
	return nil
}

func (ss *storageServer) Commit(args *storagerpc.ServerArgs,
	reply *storagerpc.ServerReply) error {
	fmt.Printf("commit called\n")
	commitVal, err := ss.PaxosHandler.PaxosCommit(args.Val)
	if err != nil {
		fmt.Printf("commit error. %s\n", err)
		return err
	} else {
		reply.Val = commitVal
		fmt.Printf("commit success\n")
	}
	return nil
}
