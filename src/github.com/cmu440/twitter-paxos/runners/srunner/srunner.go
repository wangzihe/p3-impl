package main

import (
	"fmt"
	"github.com/cmu440/twitter-paxos/storageserver"
	"net"
	"strconv"
)

var (
	port       = 9090
	configPath = "./config.txt"
	numNodes   = 2
)

func main() {
	_, err := storageserver.NewStorageServer(net.JoinHostPort("localhost", strconv.Itoa(port)), "./log.txt")
	if err != nil {
		fmt.Printf("error while creating new storage server\n")
	}
}
