package tests

import (
	//"net"
	//"net/http"
	//"net/rpc"
	//"time"
    "fmt"

	"github.com/cmu440/twitter-paxos/storageserver"
	"github.com/cmu440/twitter-paxos/paxos"
	//"github.com/cmu440/twitter-paxos/message"
	//"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

//type TestSpec struct {
//	// if rand.Float32() > rate { drop operation }
//	PingRate, prepSendRate, prepRespondRate, accSendRate, accRespondRate, commRate float32
//	// <-time.After(time.Duration(del) * time.Millisecond) before operation
//	// maybe add functionality for if del == -1 { wait random time }
//	PingDel, prepSendDel, prepRespondDel, accSendDel, accRespondDel, commDel time.Duration
//}


// basic 3 server configuration with no delays or dropped messages
func setup1Node2Fake() (bool, error) {

    // default test parameters are 0 drop rate, 0 delay, no ignores
    t := &paxos.TestSpec{}

	// SET UP SERVERS
	s1, err := storageserver.NewStorageServer(":9090", ":9095", "./configRPC1.txt", "./configMsg1.txt", *t)
	if err != nil {
		fmt.Println("failed to start server s1")
	}
    s2, err := storageserver.NewStorageServer(":9091", ":9096", "./configRPC1.txt", "./configMsg1.txt", *t)
	if err != nil {
		fmt.Println("failed to start server s2")
	}
    s3, err := storageserver.NewStorageServer(":9092", ":9097", "./configRPC1.txt", "./configMsg1.txt", *t)
	if err != nil {
		fmt.Println("failed to start server s3")
	}

	cli1, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9090"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return
	}

	cli2, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9091"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return
	}

	args1 := &storagerpc.ServerArgs{Val: "v1"}
	var reply1 storagerpc.ServerReply
	err = cli1.Call("StorageServer.Commit", args, &reply)
	if err != nil {
		fmt.Printf("error calling rpc1. %s\n", err)
	}

	args2 := &storagerpc.ServerArgs{Val: "v2"}
	var reply1 storagerpc.ServerReply
	err = cli2.Call("StorageServer.Commit", args, &reply)
	if err != nil {
		fmt.Printf("error calling rpc2. %s\n", err)
	}

    if reply1.Val != "v1" {
		fmt.Printf("incorrect value of v1: %s\n", reply1.Val)
        return false, nil
    }
    if  reply2.Val != "v2" {
		fmt.Printf("incorrect value of v2: %s\n", reply2.Val)
        return false, nil
    }
    return true, nil
}

// four nodes, A, B, C, and D
// no communication between A and D; all other nodes communicate fine
// 2. node A wants to commit value "vA" and passes prepare and accept phases,
//    supported by nodes B and C
// 4. node D tries to commit value "vD"
// 5. in the prepare phase, node B or C gives D the value "vA"
// 6. node D should commit "vA"
func failSendCommits() (bool, error) {

    DIgnore := [...]string{"localhost:9095"}
    AIgnore := [...]string{"localhost:9098"}

    tA := &paxos.TestSpec{Ignore:AIgnore}
    // default test parameters are 0 drop rate, 0 delay, no ignores
    tBC := &paxos.TestSpec{}
    tD := &paxos.TestSpec{Ignore:DIgnore}

	// SET UP REAL SERVER
	A, err := storageserver.NewStorageServer(":9090", ":9095", "./configRPC1.txt", "./configMsg1.txt", *tA)
	if err != nil {
		fmt.Println("failed to start server A")
	}
    B, err := storageserver.NewStorageServer(":9091", ":9096", "./configRPC1.txt", "./configMsg1.txt", *tBC)
	if err != nil {
		fmt.Println("failed to start server B")
	}
    C, err := storageserver.NewStorageServer(":9092", ":9097", "./configRPC1.txt", "./configMsg1.txt", *tBC)
	if err != nil {
		fmt.Println("failed to start server C")
	}
    D, err := storageserver.NewStorageServer(":9093", ":9098", "./configRPC1.txt", "./configMsg1.txt", *tD)
	if err != nil {
		fmt.Println("failed to start server D")
	}

	args1 := &storagerpc.ServerArgs{Val: "v1"}
	var reply1 storagerpc.ServerReply
	err = cli1.Call("StorageServer.Commit", args, &reply)
	if err != nil {
		fmt.Printf("error calling rpc1. %s\n", err)
	}

	args2 := &storagerpc.ServerArgs{Val: "v2"}
	var reply1 storagerpc.ServerReply
	err = cli2.Call("StorageServer.Commit", args, &reply)
	if err != nil {
		fmt.Printf("error calling rpc2. %s\n", err)
	}

    if reply1.Val != "v1" {
		fmt.Printf("incorrect value of v1: %s\n", reply1.Val)
        return false, nil
    }
    if  reply2.Val != "v1" {
		fmt.Printf("incorrect value of v2: %s\n", reply2.Val)
        return false, nil
    }
    return true, nil
}
