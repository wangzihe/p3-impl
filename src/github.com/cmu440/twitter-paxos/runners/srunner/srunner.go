package main

import (
	//"database/sql"
	"flag"
	"fmt"
	"github.com/cmu440/twitter-paxos/storageserver"
	//_ "github.com/mattn/go-sqlite3"
	//"log"
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
	_, err := storageserver.NewStorageServer(strconv.Itoa(*rpcPort), strconv.Itoa(*msgPort),
		configRPCPath, configMsgPath)
	if err != nil {
		fmt.Printf("error while creating new storage server\n")
		return
	}
	select {}
}
