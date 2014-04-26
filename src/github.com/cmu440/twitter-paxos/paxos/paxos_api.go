// paxos interface
package paxos

type Paxos interface {
	// This function is used by a node to make a new proposal
	// to other nodes. It will return true when a proposal is
	// being successfully made. Otherwise, it will return false.
	Prepare() (bool, error)

	// This function is used by a leader to do all the work in
	// accept phase. It will return true when a majority of nodes
	// reply accept-ok. Otherwise, it will return false.
	Accept() (bool, error)

	// This function is used by a leader upon receiving a prepare
	// response from an acceptor. It will react corresponding to
	// the message and make updates to the current state.
	ReceivePrepareResponse()

	// This function is used by a leader upon receiving an accept
	// response from an acceptor. It will react corresponding to
	// the message and make updates to the current state.
	ReceiveAcceptResponse()

	// This function is used by an acceptor to react to a prepare
	// message. It will react corresponding to the current state.
	ReceivePropose()

	// This function is used by an acceptor to react to an accept
	// message. It will react corresponding to the current state.
	ReceiveAccept()
}
