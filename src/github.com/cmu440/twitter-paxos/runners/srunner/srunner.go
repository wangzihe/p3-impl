package main

import (
	//"database/sql"
	"flag"
	"fmt"
	"github.com/cmu440/twitter-paxos/paxos"
	"github.com/cmu440/twitter-paxos/storageserver"
	//_ "github.com/mattn/go-sqlite3"
	//"log"
	"strconv"
	"time"
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
	/*
		db, err := sql.Open("sqlite3", "./foo.db")
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		sql := `
		create table foo (id integer not null primary key, name text);
		delete from foo;
		`
		_, err = db.Exec(sql)
		if err != nil {
			log.Printf("%q: %s\n", err, sql)
			return
		}
	*/
	testspec := paxos.TestSpec{
		PingRate:        0.0,
		PrepSendRate:    0.0,
		PrepRespondRate: 0.0,
		AccSendRate:     0.0,
		AccRespondRate:  0.0,
		CommRate:        0.0,
		PingDel:         time.Duration(0),
		PrepSendDel:     time.Duration(0),
		PrepRespondDel:  time.Duration(0),
		AccSendDel:      time.Duration(0),
		AccRespondDel:   time.Duration(0),
		CommDel:         time.Duration(0)}
	_, err := storageserver.NewStorageServer(strconv.Itoa(*rpcPort), strconv.Itoa(*msgPort),
		configRPCPath, configMsgPath, testspec)
	if err != nil {
		fmt.Printf("error while creating new storage server\n")
		return
	}
	select {}
}
