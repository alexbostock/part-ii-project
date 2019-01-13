package packet

import (
	"bytes"
	"fmt"
)

type Messagetype uint

const (
	_                             = iota
	ClientReadRequest Messagetype = iota
	ClientWriteRequest
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

	InternalTimerSignal
)

type Message struct {
	Id       int
	Src      int
	Dest     int
	DemuxKey Messagetype
	Key      []byte
	Value    []byte
	Ok       bool
}

func (m Messagetype) String() string {
	switch m {
	case ClientReadRequest:
		return "clientReadRequest"
	case ClientWriteRequest:
		return "clientWriteRequest"
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
	case InternalTimerSignal:
		return "InternalTimerSignal"
	default:
		return "UNKNOWN_MESSAGE_TYPE"
	}
}

func (m Message) Print() {
	fmt.Println()
	fmt.Println("ID", m.Id, m.Src, "->", m.Dest)
	fmt.Println(m.DemuxKey, m.Ok)
	fmt.Println(m.Key)
	fmt.Println(m.Value)
	fmt.Println()
}

func MessagesEqual(m1, m2 Message) (eq bool) {
	eq = m1.Id == m2.Id &&
		m1.Src == m2.Src &&
		m1.Dest == m2.Dest &&
		m1.DemuxKey == m2.DemuxKey &&
		bytes.Equal(m1.Key, m2.Key) &&
		bytes.Equal(m1.Value, m2.Value)

	return
}
