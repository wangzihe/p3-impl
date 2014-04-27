// Implementation of storage server
package storageserver

import (
	"bufio"
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"time"

	"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

type Status int

// Message types
const (
	serverBufSize int    = 1500 // buffer size for reading server message
	TIMEOUT       int64  = 10   // time out for ping
	RETRY         int    = 5    // number of times to retry during ping
	PING_ASK      Status = iota + 1
	PING_REPLY
	PREPARE
	PREPARE_OK
	PREPARE_REJECT
	ACCEPT
	ACCPET_OK
	ACCEPT_REJECT
	COMMIT
)

type storageServer struct {
	Hostport    string       // string for host:port
	Config      string       // path to the configuration file
	ServerPorts *list.List   // list of storage server port strings
	Ln          net.Listener // listen socket
	pingChan    chan string  // channel for ping messages between network handler and server
	LOGV        *log.Logger  // server logger
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
func (ss *storageServer) parseConfigFile() error {
	f, err := os.Open(ss.Config)
	if err != nil {
		ss.LOGV.Printf("parseConfigFile: error while opening file %s. %s\n", ss.Config, err)
		return err
	}

	reader := bufio.NewReader(f)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		ss.ServerPorts.PushBack(scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		ss.LOGV.Printf("parseConfigFile: scanner error. %s\n", err)
		return err
	}

	return nil
}

// This function read a server message from the connection.
// The function will return type of the message, value embeded
// inside the message, proposal ID and error if any.
func readMsg(conn net.Conn) (Status, string, string, error) {
	reader := bufio.NewReader(conn)
	msgBytes, err := reader.ReadBytes('\n')
	if err != nil {
		// error occurred while reading server message
		fmt.Printf("readMsg: error while reading server message. %s\n", err)
		return -1, "", "", err
	}
	// remove newline from the string read from network
	temp := string(msgBytes)
	marshalledMsg := strings.TrimSuffix(temp, "\n")
	marshalledMsgBytes := []byte(marshalledMsg)
	// unmarshal the message string and extract content
	var msg serverMsg
	err = json.Unmarshal(marshalledMsgBytes, &msg)
	if err != nil {
		fmt.Printf("readMsg: error while unmarshalling. %s\n", err)
		return -1, "", "", err
	} else {
		return msg.MsgType, msg.Value, msg.ProposalID, nil
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
/*
func (ss *storageServer) networkHandler() {
	listener := ss.Ln

	for {
		conn, err := listener.Accept()
		if err != nil {
			// listen socket is closed by the main thread
			return
		} else {
			// read server message
			msgType, _, _, errR := readMsg(conn)
			if errR != nil {
				ss.LOGV.Printf("networkHandler: error while reading msg. %s\n", errR)
			} else {
				switch msgType {
				case PING_ASK:
					// received ping message from other servers, send reply
					ss.LOGV.Printf("networkHandler: received a ping message\n")
					response := makeMsg(PING_REPLY, "", "")
					responseB, _ := wrapMsg(response)
					temp := string(responseB) + "\n"
					_, err = conn.Write([]byte(temp))
					if err != nil {
						ss.LOGV.Printf("networkHandler: error sending response. %s\n", err)
					}
					// TODO should we close conn here?
					conn.Close()
					ss.LOGV.Printf("networkHandler: send out ping response\n")
				case PING_REPLY:
					// received a ping response
					ss.LOGV.Printf("received a ping response\n")
				}
			}
		}
	}
}
*/

// This function sends ping messages to all other storage servers
// in the network to make sure they have all successfully started.
// For each storage server, it will try up to 5 times before it
// gives up and returns error. The function will return true when
// all servers are running. False otherwise.
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
// port: port string of the storage server
// config: path to the configuration file for the server. The
//         configuration file contains the list of storage servers.
func NewStorageServer(port, config string) (StorageServer, error) {
	fmt.Printf("new storage server created\n")
	server := new(storageServer)
	server.Hostport = port
	server.Config = config
	server.ServerPorts = list.New()
	// create log file for server
	filename := port + ".txt"
	logFile, _ := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	server.LOGV = log.New(logFile, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	server.LOGV.Printf("starting storage server %s\n", port)

	// parse configuration file
	server.parseConfigFile()
	server.LOGV.Printf("NewStorageServer: port is %s\n", port)
	// set up listen socket
	listenPort := ":" + port
	ln, err := net.Listen("tcp", listenPort)
	if err != nil {
		server.LOGV.Printf("NewStorageServer: error while creating listen socket. %s\n", err)
		return nil, err
	}
	server.Ln = ln

	// Wrap the storageserver before registering it for RPC
	err = rpc.RegisterName("StorageServer", storagerpc.Wrap(server))
	if err != nil {
		server.LOGV.Printf("NewStorageServer: error while registering rpc. %s\n", err)
		return nil, err
	}
	rpc.HandleHTTP()
	go http.Serve(ln, nil)

	// ping all other storage servers
	/*
		go server.networkHandler()
		if server.pingServers() == false {
			server.LOGV.Printf("some servers failed to start\n")
			server.Ln.Close()
			return nil, errors.New("not all servers exist")
		}

		fmt.Printf("finished ping\n")
	*/
	if server.pingServers() == false {
		server.LOGV.Printf("some servers failed to start\n")
		server.Ln.Close()
		return nil, errors.New("not all servers exist")
	}

	return server, nil
}

func (ss *storageServer) Ping(a, b *int) error {
	fmt.Printf("ping called\n")
	return nil
}

func (ss *storageServer) Commit(a, b *int) error {
	fmt.Printf("commit called\n")
	return nil
}
