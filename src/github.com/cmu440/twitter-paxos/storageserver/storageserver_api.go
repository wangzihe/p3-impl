// Interface for storage server
package storageserver

type StorageServer interface {
	// This function is the rpc function used by client to commit
	// changes to the storage system. It is implemented using
	// paxos algorithm.
	Commit() error
}
