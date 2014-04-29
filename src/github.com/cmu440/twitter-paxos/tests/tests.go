package main

import (
	"net"
	//"net/http"
	"net/rpc"
	//"time"
	"fmt"

	"github.com/cmu440/twitter-paxos/paxos"
	"github.com/cmu440/twitter-paxos/storageserver"
	//"github.com/cmu440/twitter-paxos/message"
	"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

// basic 3 server configuration with no delays or dropped messages
func basic3Nodes() bool {

	// default test parameters are 0 drop rate, 0 delay, no ignores
	t1 := &paxos.TestSpec{Testing: true}
	t := &paxos.TestSpec{DontRegister: true, Testing: true}

	serverReadyChan := make(chan bool)

	// SET UP SERVERS
	go func() {
		_, err := storageserver.NewStorageServer("9090", "9095", "./configRPC1.txt", "./configMsg1.txt", *t1)
		if err != nil {
			fmt.Printf("failed to start server s1 with error: %s\n", err)
		}
		serverReadyChan <- true
	}()
	go func() {
		_, err := storageserver.NewStorageServer("9091", "9096", "./configRPC1.txt", "./configMsg1.txt", *t)
		if err != nil {
			fmt.Printf("failed to start server s2 with error: %s\n", err)
		}
		serverReadyChan <- true
	}()
	go func() {
		_, err := storageserver.NewStorageServer("9092", "9097", "./configRPC1.txt", "./configMsg1.txt", *t)
		if err != nil {
			fmt.Printf("failed to start server s3 with error: %s\n", err)
		}
		serverReadyChan <- true
	}()

	// wait for all four servers to be ready
	for i := 0; i < 3; i++ {
		<-serverReadyChan
	}

	cli1, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9090"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return false
	}

	cli2, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9091"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return false
	}

	args1 := &storagerpc.ServerArgs{Val: "v1"}
	var reply1 storagerpc.ServerReply
	err = cli1.Call("StorageServer.Commit", args1, &reply1)
	if err != nil {
		fmt.Printf("error calling rpc1. %s\n", err)
		return false
	}

	args2 := &storagerpc.ServerArgs{Val: "v2"}
	var reply2 storagerpc.ServerReply
	err = cli2.Call("StorageServer.Commit", args2, &reply2)
	if err != nil {
		fmt.Printf("error calling rpc2. %s\n", err)
		return false
	}

	if reply1.Val != "v1" {
		fmt.Printf("incorrect value of v1: %s\n", reply1.Val)
		return false
	}
	if reply2.Val != "v2" {
		fmt.Printf("incorrect value of v2: %s\n", reply2.Val)
		return false
	}
	return true
}

// four nodes, A, B, C, and D
// no communication between A and D; all other nodes communicate fine
// 2. node A wants to commit value "vA" and passes prepare and accept phases,
//    supported by nodes B and C
// 4. node D tries to commit value "vD"
// 5. in the prepare phase, node B or C gives D the value "vA"
// 6. node D should commit "vA"
func disconnectTwoNodes() bool {

	DIgnore := []string{"9095"}
	AIgnore := []string{"9098"}

	tA := &paxos.TestSpec{Ignore: AIgnore, Testing: true}
	// default test parameters are 0 drop rate, 0 delay, no ignores
	tBC := &paxos.TestSpec{DontRegister: true, Testing: true}
	tD := &paxos.TestSpec{DontRegister: true, Ignore: DIgnore, Testing: true}

	serverReadyChan := make(chan bool)

	// SET UP SERVERS
	go func() {
		_, err := storageserver.NewStorageServer("9090", "9095", "./configRPC2.txt", "./configMsg2.txt", *tA)
		if err != nil {
			fmt.Printf("failed to start server A with error: %s\n", err)
		}
		serverReadyChan <- true
	}()
	go func() {
		_, err := storageserver.NewStorageServer("9091", "9096", "./configRPC2.txt", "./configMsg2.txt", *tBC)
		if err != nil {
			fmt.Printf("failed to start server B with error: %s\n", err)
		}
		serverReadyChan <- true
	}()
	go func() {
		_, err := storageserver.NewStorageServer("9092", "9097", "./configRPC2.txt", "./configMsg2.txt", *tBC)
		if err != nil {
			fmt.Printf("failed to start server C with error: %s\n", err)
		}
		serverReadyChan <- true
	}()
	go func() {
		_, err := storageserver.NewStorageServer("9093", "9098", "./configRPC2.txt", "./configMsg2.txt", *tD)
		if err != nil {
			fmt.Printf("failed to start server D with error: %s\n", err)
		}
		serverReadyChan <- true
	}()

	// wait for all four servers to be ready
	for i := 0; i < 4; i++ {
		<-serverReadyChan
	}

	cli1, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9090"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return false
	}
	cli2, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9093"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return false
	}

	args1 := &storagerpc.ServerArgs{Val: "v1"}
	var reply1 storagerpc.ServerReply
	err = cli1.Call("StorageServer.Commit", args1, &reply1)
	if err != nil {
		fmt.Printf("error calling rpc1. %s\n", err)
		return false
	}

	args2 := &storagerpc.ServerArgs{Val: "v2"}
	var reply2 storagerpc.ServerReply
	err = cli2.Call("StorageServer.Commit", args2, &reply2)
	if err != nil {
		fmt.Printf("error calling rpc2. %s\n", err)
		return false
	}

	if reply1.Val != "v1" {
		fmt.Printf("incorrect value of v1: %s\n", reply1.Val)
		return false
	}
	if reply2.Val != "v1" {
		fmt.Printf("incorrect value of v2: %s\n", reply2.Val)
		return false
	}
	return true
}

func main() {
	fmt.Println("Running test: basic3Nodes")
	if basic3Nodes() {
		fmt.Println("Passed test: basic3Nodes")
	} else {
		fmt.Println("Failed test: basic3Nodes")
	}

	//fmt.Println("Running test: disconnectTwoNodes")
	//if disconnectTwoNodes() {
	//    fmt.Println("Passed test: disconnectTwoNodes")
	//} else {
	//    fmt.Println("Failed test: disconnectTwoNodes")
	//}
}
