package simnet

import (
	"bytes"
	"fmt"
)

type messagetype uint

const (
	_                             = iota
	clientReadRequest messagetype = iota
	clientWriteRequest
	clientReadResponse
	clientWriteResponse

	nodeLockRequest
	nodeLockResponse
	nodeUnlockRequest
	nodeUnlockAck

	// Instead of nodeCommitRequest/nodeAbortRequest, use nodeUnlockRequest,
	// with the ok field indicating status

	nodeGetRequest
	nodeGetResponse
	nodePutRequest
	nodePutResponse
	nodeTimestampRequest
)

type message struct {
	id       int
	src      int
	dest     int
	demuxKey messagetype
	key      []byte
	value    []byte
	ok       bool
}

func (m messagetype) String() string {
	switch m {
	case clientReadRequest:
		return "clientReadRequest"
	case clientWriteRequest:
		return "clientWriteRequest"
	case clientReadResponse:
		return "clientReadResponse"
	case clientWriteResponse:
		return "clientWriteResponse"
	case nodeLockRequest:
		return "nodeLockRequest"
	case nodeLockResponse:
		return "nodeLockResponse"
	case nodeUnlockRequest:
		return "nodeUnlockRequest"
	case nodeUnlockAck:
		return "nodeUnlockAck"
	case nodeGetRequest:
		return "nodeGetRequest"
	case nodeGetResponse:
		return "nodeGetResponse"
	case nodePutRequest:
		return "nodePutRequest"
	case nodePutResponse:
		return "nodePutResponse"
	case nodeTimestampRequest:
		return "nodeTimestampRequest"
	default:
		return "UNKNOWN_MESSAGE_TYPE"
	}
}

func (m message) Print() {
	fmt.Println()
	fmt.Println("ID", m.id, m.src, "->", m.dest)
	fmt.Println(m.demuxKey, m.ok)
	fmt.Println(m.key)
	fmt.Println(m.value)
	fmt.Println()
}

func messagesEqual(m1, m2 message) (eq bool) {
	eq = m1.id == m2.id &&
		m1.src == m2.src &&
		m1.dest == m2.dest &&
		m1.demuxKey == m2.demuxKey &&
		bytes.Equal(m1.key, m2.key) &&
		bytes.Equal(m1.value, m2.value)

	return
}
