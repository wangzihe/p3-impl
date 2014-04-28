package main

import (
	"fmt"
	"net"
	"net/rpc"

	"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

func main() {
	cli, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9091"))
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
	return
}
