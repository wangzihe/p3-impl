// Interface for storage server
package storageserver

import "github.com/cmu440/twitter-paxos/rpc/storagerpc"

type StorageServer interface {
	Ping(*int, *int) error
	// This function is the rpc function used by client to commit
	// changes to the storage system. It is implemented using
	// paxos algorithm.
	Commit(*storagerpc.ServerArgs, *storagerpc.ServerReply) error
}
