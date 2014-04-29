// Message implementation
package message

import (
	"encoding/json"
	"strings"
)

const (
	SERVER int = iota + 1 // server message type
	PAXOS                 // paxos message type
)

// Higher level message struct that embeds server message or
// paxos message
type Message struct {
	Type int    // Type of message: either server msg or paxos msg
	Msg  string // Marshalled message
}

// Empty struct that is used as a message handler
type messageHandler struct {
}

func NewMessageHandler() MessageLib {
	return new(messageHandler)
}

func (msgLib *messageHandler) CreateMsg(msgType int,
	msg string) ([]byte, error) {
	newMsg := &Message{Type: msgType, Msg: msg}
	msgB, err := json.Marshal(*newMsg)
	if err != nil {
		return nil, err
	} else {
		temp := string(msgB) + "\n"
		return []byte(temp), nil
	}
}

func (msgLib *messageHandler) RetrieveMsg(rawMsg []byte) ([]byte,
	int, error) {
	temp := string(rawMsg)
	marshalledMsgStr := strings.TrimSuffix(temp, "\n")
	marshalledMsgBytes := []byte(marshalledMsgStr)
	var msg Message
	err := json.Unmarshal(marshalledMsgBytes, &msg)
	if err != nil {
		return nil, -1, err
	} else {
		return []byte(msg.Msg), msg.Type, nil
	}
}
