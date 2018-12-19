package simnet

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"math/rand"
	"sync"

	"github.com/alexbostock/part-ii-project/simnet/datastore"
)

type writequorum struct {
	quorumNodes   map[int]*node
	fineLock      *sync.Mutex
	clientRequest *message
	outgoing      chan message
	incoming      chan message
	aborted       bool
	localStore    datastore.Store
	localTxid     int
}

func initWriteCoord(clientRequest message, incoming, outgoing chan message, numPeers, quorumSize int, store datastore.Store) *writequorum {
	q := writequorum{
		quorumNodes:   make(map[int]*node),
		fineLock:      &sync.Mutex{},
		clientRequest: &clientRequest,
		incoming:      incoming,
		outgoing:      outgoing,
		localStore:    store,
	}

	quorumSize--

	peers := rand.Perm(numPeers)

	for _, n := range peers[:quorumSize] {
		if n == q.clientRequest.dest {
			n = numPeers
		}

		q.quorumNodes[n] = &node{}

		q.outgoing <- message{
			id:       q.clientRequest.id,
			src:      q.clientRequest.dest,
			dest:     n,
			demuxKey: nodeLockRequest,
			ok:       true,
		}
	}

	return &q
}

func (q *writequorum) nodeLocked(id int) {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	q.quorumNodes[id].locked = true

	// If all nodes are locked, continue
	for _, n := range q.quorumNodes {
		if !n.locked {
			return
		}
	}

	for id := range q.quorumNodes {
		q.outgoing <- message{
			id:       q.clientRequest.id,
			src:      q.clientRequest.dest,
			dest:     id,
			demuxKey: nodeTimestampRequest,
			key:      q.clientRequest.key,
			ok:       true,
		}
	}
}

func (q *writequorum) handleRead(id int, key, value []byte) {
	q.quorumNodes[id].value = value

	// If all nodes have returned a value, collect timestamps and continue
	for _, node := range q.quorumNodes {
		if node.value == nil {
			return
		}
	}

	var latestTimestamp uint64

	localVal := q.localStore.Get(key)
	if len(localVal) > 0 {
		latestTimestamp, _ = decodeTimestampVal(localVal)
	}

	for _, node := range q.quorumNodes {
		timestamp, _ := decodeTimestampVal(node.value)
		if timestamp > latestTimestamp {
			latestTimestamp = timestamp
		}
	}

	timestampBytes := make([]byte, 8)

	value = make([]byte, 12)
	binary.BigEndian.PutUint64(timestampBytes, latestTimestamp+1)
	base64.StdEncoding.Encode(value, timestampBytes)

	value = append(value, q.clientRequest.value...)

	q.localTxid = q.localStore.Put(key, value)

	for id := range q.quorumNodes {
		q.outgoing <- message{
			id:       q.clientRequest.id,
			src:      q.clientRequest.dest,
			dest:     id,
			demuxKey: nodePutRequest,
			key:      key,
			value:    value,
			ok:       true,
		}
	}
}

func (q *writequorum) nodeReturned(id int, key, value []byte, ok bool) {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	if q.aborted {
		return
	}

	if !bytes.Equal(key, q.clientRequest.key) {
		return
	}

	if q.quorumNodes[id] == nil {
		return
	}

	// !ok means a no vote, so abort
	if !ok {
		q.abort(true)
		return
	}

	// nil value used to signal nodePutResponse rather than nodeGetResponse
	if value != nil {
		q.handleRead(id, key, value)
		return
	}

	q.quorumNodes[id].voted = true

	// If all nodes have voted yes, continue
	for _, n := range q.quorumNodes {
		if !n.voted {
			return
		}
	}

	for n := range q.quorumNodes {
		q.outgoing <- message{
			id:       q.clientRequest.id,
			src:      q.clientRequest.dest,
			dest:     n,
			demuxKey: nodeUnlockRequest,
			ok:       true,
		}
	}
}

// nodeUnlocked actually means nodeCommitted, in this case
func (q *writequorum) nodeUnlocked(id int) {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	if q.quorumNodes[id] == nil {
		return
	}

	q.quorumNodes[id].unlocked = true

	// If all nodes have committed, write to the local store
	for _, n := range q.quorumNodes {
		if !n.unlocked {
			return
		}
	}

	q.localStore.Commit(q.clientRequest.key, q.localTxid)

	q.incoming <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.dest,
		demuxKey: nodeUnlockRequest,
		ok:       true,
	}

	q.outgoing <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.src,
		demuxKey: clientWriteResponse,
		key:      q.clientRequest.key,
		value:    q.clientRequest.value,
		ok:       true,
	}
}

func (q *writequorum) abort(internalCall bool) {
	if !internalCall {
		q.fineLock.Lock()
		defer q.fineLock.Unlock()
	}

	if q.aborted {
		return
	}

	// Unlock messages are used as abort messages
	for id := range q.quorumNodes {
		q.outgoing <- message{
			id:       q.clientRequest.id,
			src:      q.clientRequest.dest,
			dest:     id,
			demuxKey: nodeUnlockRequest,
			ok:       false,
		}
	}

	q.outgoing <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.src,
		demuxKey: clientWriteResponse,
		key:      q.clientRequest.key,
		value:    q.clientRequest.value,
		ok:       false,
	}

	q.incoming <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.dest,
		demuxKey: nodeUnlockRequest,
		ok:       false,
	}

	q.aborted = true
}
