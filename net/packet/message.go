// Package packet implements the format of every message in the simulated
// network.
package packet

import (
	"bytes"
	"fmt"
)

// A MessageType is an enum indicating the type of a network message. The prefix
// indicates a message's purpose:
// Client: for client-node communications.
// Node: for node-node message processing transactions.
// Internal: for timing signals within a node.
// Election: used for leadership elections.
// Control: used to control the simulation.
type Messagetype uint

const (
	_                                    = iota
	ClientReadRequest        Messagetype = iota
	ClientWriteRequest                   // Blind overwrite
	ClientStrongWriteRequest             // Write at timestamp (or fail)
	ClientReadResponse
	ClientWriteResponse

	NodeLockRequest
	NodeLockRequestNoTimeout
	NodeLockResponse
	NodeUnlockRequest
	NodeUnlockAck

	// Instead of nodeCommitRequest/nodeAbortRequest, use nodeUnlockRequest,
	// with the ok field indicating status

	NodeGetRequest
	NodeGetResponse
	NodePutRequest
	NodePutResponse
	NodeTimestampRequest

	NodeBackgroundWriteRequest
	NodeBackgroundWriteResponse

	InternalTimerSignal
	InternalHeartbeat
	InternalLeaderQuery

	ElectionElect
	ElectionCoordinator
	ElectionAck

	ControlFail
	ControlRecover
)

// A Message represents 1 simulated network message.
// Fields:
// Id: transaction ID (should unique for every transaction)
// Src: the source address (ID of a Dbnode or Client)
// Dest: the destination address (ID of a Dbnode or Client)
// DemuxKey: the type of message (see MessageType)
// Key: a database key
// Value: a database value
// Timestamp: a Lamport clock value for a database value
// Ok: false iff an error has occurred
type Message struct {
	Id        int
	Src       int
	Dest      int
	DemuxKey  Messagetype
	Key       []byte
	Value     []byte
	Timestamp uint64
	Ok        bool
}

// String converts a MessageType to a string
func (m Messagetype) String() string {
	switch m {
	case ClientReadRequest:
		return "clientReadRequest"
	case ClientWriteRequest:
		return "clientWriteRequest"
	case ClientStrongWriteRequest:
		return "clientStrongWriteRequest"
	case ClientReadResponse:
		return "clientReadResponse"
	case ClientWriteResponse:
		return "clientWriteResponse"
	case NodeLockRequest:
		return "nodeLockRequest"
	case NodeLockRequestNoTimeout:
		return "nodeLockRequestNoTimeout"
	case NodeLockResponse:
		return "nodeLockResponse"
	case NodeUnlockRequest:
		return "nodeUnlockRequest"
	case NodeUnlockAck:
		return "nodeUnlockAck"
	case NodeGetRequest:
		return "nodeGetRequest"
	case NodeGetResponse:
		return "nodeGetResponse"
	case NodePutRequest:
		return "nodePutRequest"
	case NodePutResponse:
		return "nodePutResponse"
	case NodeTimestampRequest:
		return "nodeTimestampRequest"
	case ElectionElect:
		return "electionElect"
	case ElectionCoordinator:
		return "electionCoordinator"
	case ElectionAck:
		return "electionAck"
	case InternalTimerSignal:
		return "internalTimerSignal"
	case InternalHeartbeat:
		return "internalHeartbeat"
	default:
		return "UNKNOWN_MESSAGE_TYPE"
	}
}

// Print (non-atomically) prints a Message
func (m Message) Print() {
	fmt.Println(m)
}

// MessagesEqual returns true iff two messages are identical (comparing keys
// and values using bytes.Equal)
func MessagesEqual(m1, m2 Message) (eq bool) {
	eq = m1.Id == m2.Id &&
		m1.Src == m2.Src &&
		m1.Dest == m2.Dest &&
		m1.DemuxKey == m2.DemuxKey &&
		bytes.Equal(m1.Key, m2.Key) &&
		bytes.Equal(m1.Value, m2.Value)

	return
}
