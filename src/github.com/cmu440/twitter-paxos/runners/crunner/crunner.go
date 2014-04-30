package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"

	"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

func main() {
	clients := make([]*rpc.Client, 3)
	cli1, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9091"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return
	}
	cli2, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9092"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return
	}
	cli3, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9093"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return
	}
	clients[0] = cli1
	clients[1] = cli2
	clients[2] = cli3

	for index := 0; index < 1000; index++ {
		cli := clients[rand.Int()%3]
		val := strconv.Itoa(index)
		args := &storagerpc.ServerArgs{Val: val}
		var reply storagerpc.ServerReply
		err = cli.Call("StorageServer.Commit", args, &reply)
		if err != nil {
			fmt.Printf("error calling rpc. %s\n", err)
		}
		/*
			for reply.Val != val {
				args2 := &storagerpc.ServerArgs{Val: val}
				var reply2 storagerpc.ServerReply
				err = cli.Call("StorageServer.Commit", args, &reply2)
				if err != nil {
					fmt.Printf("error calling rpc. %s\n", err)
				}
			}
		*/
	}

	return
}
