package simnet

import (
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
			node.quorumCoordinator.abort(false)
		}
	case nodeGetResponse:
		node.quorumCoordinator.nodeReturned(msg.src, msg.key, msg.value)
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
			// TODO
			node.outgoing <- message{
				msg.id,
				node.id,
				msg.src,
				clientWriteResponse,
				msg.key,
				msg.value,
				false,
			}

			// Remember to remove when writes are implemented
			node.lock.Unlock(msg.id)
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
	time.Sleep(timeout / 5)

	node.incoming <- message{
		id:       id,
		src:      node.id,
		dest:     node.id,
		demuxKey: nodeUnlockRequest,
		ok:       true,
	}
}
