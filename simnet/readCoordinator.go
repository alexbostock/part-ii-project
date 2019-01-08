package simnet

import (
	"bytes"
	"math/rand"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/simnet/datastore"
)

type readquorum struct {
	quorumNodes       map[int]*node
	fineLock          *sync.Mutex
	clientRequest     *message
	outgoing          chan message
	localSignalChan   chan bool
	aborted           bool
	lastOperationTime time.Time
	localStore        datastore.Store
}

func initReadCoord(clientRequest message, signalChan chan bool, outgoing chan message, numPeers, quorumSize int, store datastore.Store) *readquorum {
	q := readquorum{
		quorumNodes:     make(map[int]*node),
		fineLock:        &sync.Mutex{},
		clientRequest:   &clientRequest,
		localSignalChan: signalChan,
		outgoing:        outgoing,
		localStore:      store,
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

	if q.aborted {
		return
	}

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

func (q *readquorum) nodeReturned(id int, key, value []byte, ok bool) {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	q.lastOperationTime = time.Now()

	if q.aborted {
		return
	}

	if !bytes.Equal(key, q.clientRequest.key) {
		return
	}

	if q.quorumNodes[id] == nil {
		return
	}

	if !ok || value == nil {
		q.abort(true)
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

	// Unlock self
	q.localSignalChan <- true
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

	q.outgoing <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.src,
		demuxKey: clientReadResponse,
		key:      q.clientRequest.key,
		ok:       false,
	}

	// Unlock self
	q.localSignalChan <- false

	q.aborted = true
}

func (q *readquorum) respondError() {
	q.fineLock.Lock()
	defer q.fineLock.Unlock()

	q.outgoing <- message{
		id:       q.clientRequest.id,
		src:      q.clientRequest.dest,
		dest:     q.clientRequest.src,
		demuxKey: clientReadResponse,
		key:      q.clientRequest.key,
		ok:       false,
	}
}
