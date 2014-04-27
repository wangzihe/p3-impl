package main

import (
	"flag"
	"fmt"
	"github.com/cmu440/twitter-paxos/storageserver"
	"strconv"
)

var (
	rpcPort       = flag.Int("rpc", 9090, "Storage server rpc port number")
	msgPort       = flag.Int("msg", 9099, "Storage server msg port number")
	configRPCPath = "./configRPC.txt"
	configMsgPath = "./configMsg.txt"
)

func main() {
	flag.Parse()
	fmt.Printf("number of args is %d\n", flag.NArg())
	_, err := storageserver.NewStorageServer(strconv.Itoa(*rpcPort), strconv.Itoa(*msgPort),
		configRPCPath, configMsgPath)
	if err != nil {
		fmt.Printf("error while creating new storage server\n")
		return
	}
	select {}
}
