package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"

	"github.com/cmu440/twitter-paxos/rpc/storagerpc"
)

var (
	clients       = make([]*rpc.Client, 3)
	communication = make(chan struct{}, 3)
)

func runCommit(index, number int) {
	for i := 0; i < number; i++ {
		cli := clients[rand.Int()%2]
		val := strconv.Itoa(rand.Int())
		args := &storagerpc.ServerArgs{Val: val}
		var reply storagerpc.ServerReply
		err := cli.Call("StorageServer.Commit", args, &reply)
		if err != nil {
			fmt.Printf("error calling rpc. %s\n", err)
		}
	}
	communication <- struct{}{}
}

func main() {

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

	for index := 0; index < 20; index++ {
		go runCommit(index, 20)
	}
	fmt.Printf("finished running\n")

	for index := 0; index >= 0; index++ {
		<-communication
		fmt.Printf("client %d finished \n", index)
		if index == 19 {
			return
		}
	}

	return
}
