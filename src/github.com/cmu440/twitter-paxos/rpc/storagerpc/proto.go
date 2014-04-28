// This file contains server argument and reply structs used
// by rpc.
package storagerpc

type ServerArgs struct {
	Val string // value to be commited
}

type ServerReply struct {
	Val string // value that gets committed
}
