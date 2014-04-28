// Message interface
package message

type MessageLib interface {
	// This function creates a message by embedding either a server
	// message or a paxos message into a higher level message type.
	// The higher level message is ended with a newline character.
	CreateMsg(int, []byte) ([]byte, error)

	// This function retrieves a message from the higher level
	// message type received throught network. It returns the byte
	// array of the lower level message embeded inside. It also
	// retrieves type of message embeded inside the higher level
	// message type. It can be either a server message or a paxos
	// message.
	RetrieveMsg([]byte) ([]byte, int, error)
}
