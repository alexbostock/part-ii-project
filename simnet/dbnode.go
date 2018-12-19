package simnet

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexbostock/part-ii-project/simnet/datastore"
	"github.com/alexbostock/part-ii-project/simnet/quorumlock"
)

type nodestate struct {
	id                int          // A unique node identifier
	incoming          chan message // Streams of network messages
	outgoing          chan message
	clientReqQueue    chan message // Holds client request to avoid blocking incoming
	lock              *quorumlock.Lock
	store             datastore.Store
	quorumCoordinator coordinator

	numPeers   int
	readQSize  int
	writeQSize int

	uncommitedKey  []byte
	uncommitedTxid int
}

func startNode(n int, id int, incoming, outgoing chan message, lockTimeout time.Duration) {
	// TODO: quorum sizes as parameters
	state := nodestate{
		id:                id,
		incoming:          incoming,
		outgoing:          outgoing,
		clientReqQueue:    make(chan message, 25),
		lock:              quorumlock.New(lockTimeout),
		store:             datastore.CreateStore(filepath.Join("data", strconv.Itoa(id))),
		quorumCoordinator: nil,

		numPeers:   n - 1,
		writeQSize: n/2 + 1,
		readQSize:  n/2 + 1,

		uncommitedKey: nil,
	}

	go state.handleRequests(lockTimeout)

	for msg := range incoming {
		if msg.dest != id {
			log.Print("Misdelivered message for ", msg.dest, " delivered to ", id)
			continue
		}
		switch msg.demuxKey {
		case clientReadRequest:
			state.clientReqQueue <- msg
		case clientWriteRequest:
			state.clientReqQueue <- msg
		case nodeLockRequest:
			go state.acquireLock(msg, lockTimeout)
		case nodeLockResponse:
			state.coordinatorHandle(msg)
		case nodeUnlockRequest:
			if state.uncommitedTxid > 0 {
				if msg.ok {
					state.store.Commit(state.uncommitedKey, state.uncommitedTxid)
				} else {
					// Rollback actually means nothing to the store
				}

				state.uncommitedKey = nil
				state.uncommitedTxid = 0
			}

			state.lock.Unlock(msg.id)

			if id != msg.src {
				outgoing <- message{
					id:       msg.id,
					src:      id,
					dest:     msg.src,
					demuxKey: nodeUnlockAck,
					ok:       true,
				}
			}
		case nodeUnlockAck:
			state.coordinatorHandle(msg)
		case nodeGetRequest:
			if state.lock.HeldBy(msg.id) {
				val := state.store.Get(msg.key)

				outgoing <- message{
					id:       msg.id,
					src:      id,
					dest:     msg.src,
					demuxKey: nodeGetResponse,
					key:      msg.key,
					value:    val,
					ok:       val == nil,
				}
			} else {
				outgoing <- message{
					id:       msg.id,
					src:      id,
					dest:     msg.src,
					demuxKey: nodeGetResponse,
					key:      msg.key,
					ok:       false,
				}
			}
		case nodeGetResponse:
			state.coordinatorHandle(msg)
		case nodeTimestampRequest:
			// Must always respond with nodeGetResponse, ok: true
			var val []byte

			if state.lock.HeldBy(msg.id) {
				val = state.store.Get(msg.key)
				if len(val) == 0 {
					// Respond with 0 timestamp on failure
					val = make([]byte, 12)
					var ts uint64 = 0
					tsBytes := make([]byte, 8)
					binary.BigEndian.PutUint64(tsBytes, ts)
					base64.StdEncoding.Encode(val, tsBytes)
				}
			} else {
				// Respond with 0 timestamp on failure
				val = make([]byte, 12)
				var ts uint64 = 0
				tsBytes := make([]byte, 8)
				binary.BigEndian.PutUint64(tsBytes, ts)
				base64.StdEncoding.Encode(val, tsBytes)
			}

			outgoing <- message{
				id:       msg.id,
				src:      id,
				dest:     msg.src,
				demuxKey: nodeGetResponse,
				key:      msg.key,
				value:    val,
				ok:       true,
			}
		case nodePutRequest:
			if state.lock.HeldBy(msg.id) {
				state.uncommitedTxid = state.store.Put(msg.key, msg.value)
				if state.uncommitedTxid > 0 {
					state.uncommitedKey = msg.key
				}
				outgoing <- message{
					id:       msg.id,
					src:      id,
					dest:     msg.src,
					demuxKey: nodePutResponse,
					key:      msg.key,
					value:    msg.value,
					ok:       state.uncommitedTxid > 0,
				}
			} else {
				outgoing <- message{
					id:       msg.id,
					src:      id,
					dest:     msg.src,
					demuxKey: nodePutResponse,
					key:      msg.key,
					value:    msg.value,
					ok:       false,
				}
			}
		case nodePutResponse:
			state.coordinatorHandle(msg)
		default:
			log.Printf("Unexpected message type received %+v", msg)
		}
	}
}

func (node *nodestate) coordinatorHandle(msg message) {
	if node.quorumCoordinator == nil || !node.lock.HeldBy(msg.id) {
		return
	}

	switch msg.demuxKey {
	case nodeLockResponse:
		if msg.ok {
			node.quorumCoordinator.nodeLocked(msg.src)
		} else {
			node.quorumCoordinator.nodeUnlocked(msg.src)
			node.quorumCoordinator.abort(false)
		}
	case nodeGetResponse:
		node.quorumCoordinator.nodeReturned(msg.src, msg.key, msg.value, msg.ok)
	case nodePutResponse:
		node.quorumCoordinator.nodeReturned(msg.src, msg.key, nil, msg.ok)
	case nodeUnlockAck:
		node.quorumCoordinator.nodeUnlocked(msg.src)
	default:
		log.Fatalf("Message misdirected to coordinatorHandle %+v", msg)
	}
}

func (node *nodestate) handleRequests(lockTimeout time.Duration) {
	for msg := range node.clientReqQueue {
		node.lock.Lock(msg.id)
		fmt.Println(node.id, "handling", msg.id)

		switch msg.demuxKey {
		case clientReadRequest:
			node.quorumCoordinator = initReadCoord(msg, node.incoming, node.outgoing, node.numPeers, node.readQSize, node.store)
		case clientWriteRequest:
			node.quorumCoordinator = initWriteCoord(msg, node.incoming, node.outgoing, node.numPeers, node.writeQSize, node.store)
		}

		time.Sleep(lockTimeout)
	}
}

func (node *nodestate) acquireLock(msg message, timeout time.Duration) {
	locked := node.lock.Lock(msg.id)

	node.outgoing <- message{
		id:       msg.id,
		src:      node.id,
		dest:     msg.src,
		demuxKey: nodeLockResponse,
		ok:       locked,
	}

	go node.startLockTimer(msg.id, timeout)
}

func (node *nodestate) startLockTimer(id int, timeout time.Duration) {
	time.Sleep(timeout / 2)

	node.incoming <- message{
		id:       id,
		src:      node.id,
		dest:     node.id,
		demuxKey: nodeUnlockRequest,
		ok:       true,
	}
}
