package main

import (
	"fmt"
	"net"
	"net/rpc"
)

func main() {
	cli, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", "9091"))
	if err != nil {
		fmt.Printf("error dialing rpc. %s\n", err)
		return
	}
	var a int = 1
	var b int = 2
	err = cli.Call("StorageServer.Commit", &a, &b)
	if err != nil {
		fmt.Printf("error calling rpc. %s\n", err)
	}
	return
}
