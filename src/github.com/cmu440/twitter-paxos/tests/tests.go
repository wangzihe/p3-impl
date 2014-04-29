package tests

import (
	"net"
	"net/http"
	"net/rpc"
	//"time"

	"github.com/cmu440/twitter-paxos/storageserver"
	//"github.com/cmu440/twitter-paxos/paxos"
	"github.com/cmu440/twitter-paxos/message"
	"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

//type fakeServer struct {
//}

// basic 3 server configuration
//func setup1Node2Fake() (bool, error) {
//
//    // SET UP FAKE SERVERS
//	// set up listen sockets
//	rpcLn1, err := net.Listen("tcp", ":9091")
//	rpcLn2, err := net.Listen("tcp", ":9092")
//	if err != nil {
//		return false, err
//	}
//	msgLn1, err := net.Listen("tcp", ":9096")
//	msgLn2, err := net.Listen("tcp", ":9097")
//	if err != nil {
//		return false, err
//	}
//    // TODO: figure out how to register
//	err = rpc.RegisterName("StorageServer", storagerpc.Wrap(server))
//	if err != nil {
//		return nil, err
//	}
//	rpc.HandleHTTP()
//	go http.Serve(rpcLn1, nil)
//	go http.Serve(rpcLn2, nil)
//	go networkHandler(msgLn1)
//	go networkHandler(msgLn2)
//
//
//    // SET UP REAL SERVER
//    s1, err = NewStorageServer(":9090", ":9095", "./configRPC.txt", "./configMsg.txt")
//    if err != nil {
//        fmt.Println("failed to start server")
//    }
//
//    // ping the real storage server to make sure its running
//		var fail bool = true
//		for index := 0; index < 5; index++ {
//	        cli, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9090"))
//			if err != nil {
//				time.Sleep(time.Duration(1) * time.Second)
//				continue
//			} else {
//				var a int = 1
//				var b int = 2
//				err = cli.Call("StorageServer.Ping", &a, &b)
//				if err != nil {
//					continue
//				} else {
//					fail = false
//					break
//				}
//			}
//		}
//		if fail == true {
//			return false
//		}
//    return true, nil
//}
//
//func fakePing(a, b *int) error {
//	fmt.Printf("fakePing called\n")
//	return nil
//}
//
//// This is the handler to handle messages received by the server.
//func fakeNetworkHandler() {
//    MsgHandler = message.NewMessageHandler()
//	listener := ss.MsgListener
//
//	for {
//		// read server message
//		msgB, msgType, errR := readMsg(conn)
//
//		if errR != nil {
//			fmt.Printf("networkHandler: error while reading msg. %s\n", errR)
//		} else {
//			switch msgType {
//			case message.SERVER:
//				// received a server message
//				ss.parseServerMsg(msgB)
//				conn.Close()
//			case message.PAXOS:
//				// received a paxos message
//				// TODO use go routine to handle paxos message
//				go ss.PaxosHandler.Interpret_message(msgB)
//				conn.Close()
//			}
//		}
//	}
//}
//
//// This function read a high-level message from the connection.
//// This message can embed a server message or a paxos message.
//// The function will return the byte array for the message embeded
//// inside the high-level message, message type and error
//func readMsg(conn net.Conn) ([]byte, int, error) {
//	reader := bufio.NewReader(conn)
//	msgBytes, err := reader.ReadBytes('\n')
//	if err != nil {
//		fmt.Printf("readMsg: error while reading server message. %s\n", err)
//		return nil, -1, err
//	}
//	generalMsgB, msgType, err := ss.MsgHandler.RetrieveMsg(msgBytes)
//	if err != nil {
//		return nil, -1, err
//	} else {
//		return generalMsgB, msgType, nil
//	}
//}

// basic 3 server configuration
func setup1Node2Fake() (bool, error) {

	// SET UP REAL SERVER
	s1, err = NewStorageServer(":9090", ":9095", "./configRPC.txt", "./configMsg.txt")
    s2, err = NewStorageServer(":9091", ":9096", "./configRPC.txt", "./configMsg.txt")
    s3, err = NewStorageServer(":9092", ":9097", "./configRPC.txt", "./configMsg.txt")
	if err != nil {
		fmt.Println("failed to start server")
	}

	cli, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9090"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return
	}

	args := &storagerpc.ServerArgs{Val: "hello world"}
	var reply storagerpc.ServerReply
	err = cli.Call("StorageServer.Commit", args, &reply)
	if err != nil {
		fmt.Printf("error calling rpc. %s\n", err)
	}
}
