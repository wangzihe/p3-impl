// Implementation of storage server
package storageserver

import (
	"fmt"
)

type storageServer struct {
	hostport string // string for host:port
	config   string // path to the configuration file
}

// This function creates a new storage server.
// serverHostPort: host-port string of the storage server
// config: path to the configuration file for the server. The
//         configuration file contains the list of storage servers.
func NewStorageServer(serverHostPort, config string) (StorageServer, error) {
	fmt.Printf("new storage server created\n")
	return nil, nil
}
