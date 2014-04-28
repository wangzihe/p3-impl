// paxos interface
package paxos

type PaxosStates interface {
	// This function is used by a node to make a new proposal
	// to other nodes. It will return true when a proposal is
	// being successfully made. Otherwise, it will return false.
	Prepare() (bool, error)

	// This function is used by a leader to do all the work in
	// accept phase. It will return true when a majority of nodes
	// reply accept-ok. Otherwise, it will return false.
	Accept(string) (bool, error)

	// This function is used by a leader to commit a value.
	// It will return an error if something fails. Otherwise,
	// it will just return a nil error message.
	//CommitVal() error

	PaxosCommit(string) (string, error)

	Interpret_message([]byte)
}
