// This file provides a type-safe wrapper that should be used to register the
// storage server to receive RPCs from a client.

package storagerpc

type RemoteStorageServer interface {
	Commit(*int, *int) error
	Ping(*int, *int) error
}

type StorageServer struct {
	RemoteStorageServer
}

func Wrap(s RemoteStorageServer) RemoteStorageServer {
	return &StorageServer{s}
}
