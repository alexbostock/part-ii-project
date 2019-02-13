package dbnode

import (
	"log"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/packet"
)

// A propagater quietly streams write requests to other nodes and tracks
// responses to ensure that at least n - V_R + 1 nodes eventually receive every
// value.
type propagater struct {
	id           int
	n            int
	criticalSize int
	outgoing     chan packet.Message
	transactions map[int]*transaction

	// Channel used to signal for the main loop to send requests
	timer chan bool
	// (triggered when a new transaction is added, add every second)

	lock sync.Mutex
}

// A transaction represents a single put transaction, including the data written,
// the number of nodes known to have stored this transaction, and the set of
// nodes known to have stored this transaction.
type transaction struct {
	key               []byte
	value             []byte
	timestamp         uint64
	numConfirmedNodes int
	nodes             map[int]bool
}

// newPropagater instantiates propagator, including starting its clock and main
// loop. Its arguments are this node's id, the total number of nodes, the read
// quorum size V_R, and the outgoing network link for this node.
func newPropagater(id, numNodes, rqs int, outgoing chan packet.Message) *propagater {
	p := &propagater{
		id:           id,
		n:            numNodes,
		criticalSize: numNodes - rqs + 1,
		outgoing:     outgoing,
		transactions: make(map[int]*transaction),

		timer: make(chan bool),
	}

	go p.streamWrites()

	return p
}

// startClock sends a clock signal on p.timer every second. This should run
// asynchronously.
func (p *propagater) startClock() {
	for {
		time.Sleep(time.Second)
		p.timer <- true
	}
}

// streamWrites is the main loop. It iterates every time there is a signal on
// p.timer.
func (p *propagater) streamWrites() {
	for _ = range p.timer {
		p.lock.Lock()

		for id, t := range p.transactions {
			if t.numConfirmedNodes >= p.criticalSize {
				delete(p.transactions, id)
				continue
			}

			for node := 0; node < p.n; node++ {
				if !t.nodes[node] {
					p.outgoing <- packet.Message{
						Id:        id,
						Src:       p.id,
						Dest:      node,
						DemuxKey:  packet.NodeBackgroundWriteRequest,
						Key:       t.key,
						Value:     t.value,
						Timestamp: t.timestamp,
						Ok:        true,
					}
				}
			}
		}

		p.lock.Unlock()
	}
}

// propagateTransaction adds a transaction to the propagater so that the main
// loop will begin propagating it. Its arguments are the transaction id, the
// set of nodes involved in the atomic write transaction (including this node,
// the coordinator), and the values stored.
func (p *propagater) propagateTransaction(id int, quorumMembers map[int]packet.Message, key, value []byte, timestamp uint64) {
	p.lock.Lock()
	defer p.lock.Unlock()

	t := &transaction{
		key:               key,
		value:             value,
		timestamp:         timestamp,
		numConfirmedNodes: len(quorumMembers),
		nodes:             make(map[int]bool),
	}

	for node, _ := range quorumMembers {
		t.nodes[node] = true
	}

	p.transactions[id] = t
	p.timer <- true
}

// response should be called whenever the dbnode receives a
// NodeBackgroundWriteResponse. In addition, whenever the dbnode receives a
// NodeBackgroundWriteResponse with Ok == false, it should store the key, value
// and timestamp given.
func (p *propagater) response(msg packet.Message) {
	if msg.DemuxKey != packet.NodeBackgroundWriteResponse {
		log.Fatal("Misdelivered message to propagator", msg)
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	t := p.transactions[msg.Id]
	if t == nil {
		return
	}

	// msg.Ok means value updated successfully
	if msg.Ok && !t.nodes[msg.Src] {
		t.nodes[msg.Src] = true
		t.numConfirmedNodes++

		if t.numConfirmedNodes >= p.criticalSize {
			delete(p.transactions, msg.Id)
		}
	}

	// !msg.Ok means the response contains a newer value and timestamp for
	// this key. This means that we should stop propagating the now stale
	// value. It is dbnode's job to store the new value.
	if !msg.Ok {
		if msg.Timestamp <= t.timestamp {
			log.Fatal("Conflicting timestamps", msg, t)
		}

		delete(p.transactions, msg.Id)
	}
}
