package main

import (
	"flag"
	"fmt"
	"github.com/cmu440/twitter-paxos/storageserver"
	"strconv"
)

var (
	port       = flag.Int("port", 9090, "Storage server port number")
	configPath = "./config.txt"
	numNodes   = 2
)

func main() {
	flag.Parse()
	fmt.Printf("number of args is %d\n", flag.NArg())
	_, err := storageserver.NewStorageServer(strconv.Itoa(*port), "./config.txt")
	if err != nil {
		fmt.Printf("error while creating new storage server\n")
		return
	}
	select {}
}
