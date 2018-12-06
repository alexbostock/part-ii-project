package simnet

import (
	"bytes"
	"encoding/binary"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/simnet/datastore"
)

type node struct {
	locked   bool
	value    []byte
	unlocked bool
}

type readquorum struct {
	quorumNodes       map[int]*node
	fineLock          *sync.Mutex
	clientRequest     *message
	outgoing          chan message
	incoming          chan message
	aborted           bool
	lastOperationTime time.Time
	localStore        datastore.Store
}

func (q readquorum) correctId(id int) bool {
	return id == q.clientRequest.id
}

func initReadCoord(clientRequest message, incoming, outgoing chan message, numPeers, quorumSize int, store datastore.Store) *readquorum {
	q := readquorum{
		quorumNodes:   make(map[int]*node),
		fineLock:      &sync.Mutex{},
		clientRequest: &clientRequest,
		incoming:      incoming,
		outgoing:      outgoing,
		localStore:    store,
	}

	// The coordinator is one member of the quorum
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

	q.lastOperationTime = time.Now()

	go q.setTimer()

	return &q
}

func (q *readquorum) setTimer() {
	// TODO: timeout length as parameter
	timeoutLength := 500 * time.Millisecond

	for {
		q.fineLock.Lock()

		if q.aborted {
			return
		}

		if time.Now().Sub(q.lastOperationTime) > timeoutLength {
			q.abort(true)
		}

		q.fineLock.Unlock()

		time.Sleep(timeoutLength / 10)
	}
}

func (q *readquorum) nodeLocked(id int) {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	q.lastOperationTime = time.Now()

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
			demuxKey: nodeGetRequest,
			key:      q.clientRequest.key,
			ok:       true,
		}
	}
}

func (q *readquorum) nodeReturned(id int, key, value []byte) {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	q.lastOperationTime = time.Now()

	if q.aborted {
		return
	}

	if !bytes.Equal(key, q.clientRequest.key) {
		return
	}

	if value == nil {
		q.abort(true)
		return
	}

	if q.quorumNodes[id] == nil {
		return
	}

	q.quorumNodes[id].value = value

	// If all nodes have returned, continue
	for _, n := range q.quorumNodes {
		if n.value == nil {
			return
		}
	}

	timestamp, val := decodeTimestampVal(q.localStore.Get(key))

	for id, n := range q.quorumNodes {
		// Find the most recent value
		t, v := decodeTimestampVal(n.value)
		if t >= timestamp {
			timestamp = t
			val = v
		}

		// Unlock each node in the quorum
		q.outgoing <- message{
			id:       q.clientRequest.id,
			src:      q.clientRequest.dest,
			dest:     id,
			demuxKey: nodeUnlockRequest,
			ok:       true,
		}
	}

	// Return to the client
	q.outgoing <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.src,
		demuxKey: clientReadResponse,
		key:      q.clientRequest.key,
		value:    val,
		ok:       true,
	}

	// Send unlock request to self
	q.incoming <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.dest,
		demuxKey: nodeUnlockRequest,
		ok:       true,
	}
}

func (q *readquorum) nodeUnlocked(id int) {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	q.lastOperationTime = time.Now()

	q.quorumNodes[id].unlocked = true
}

func (q *readquorum) abort(internalCall bool) {
	if !internalCall {
		q.fineLock.Lock()
		defer q.fineLock.Lock()
	}

	if q.aborted {
		return
	}

	// Unlock the whole quorum
	for id := range q.quorumNodes {
		q.outgoing <- message{
			id:       q.clientRequest.id,
			src:      q.clientRequest.dest,
			dest:     id,
			demuxKey: nodeUnlockRequest,
			ok:       false,
		}
	}

	// Send error response to the client
	q.outgoing <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.src,
		demuxKey: clientReadResponse,
		key:      q.clientRequest.key,
		ok:       false,
	}

	// Send unlock request to self
	q.incoming <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.dest,
		demuxKey: nodeUnlockRequest,
		ok:       false,
	}

	q.aborted = true
}

func decodeTimestampVal(encoded []byte) (timestamp uint64, value []byte) {
	// First 8 bytes are Lamport timestamp

	if len(encoded) < 8 {
		log.Fatal("Invalid value stored: value prefix must be a 64 bit Lamport timestamp")
	}

	timestamp = binary.BigEndian.Uint64(encoded[:8])
	value = encoded[8:]

	return
}
