package repeater

import (
	"log"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/packet"
)

// A system to repeatedly send requests until they are acknowledged

type Repeater struct {
	outgoing   chan packet.Message
	timeout    time.Duration
	numRetries int
	lock       sync.Mutex

	// nodeID -> txID -> messageType -> bool
	unackedReqs map[int]map[int]map[packet.Messagetype]bool
}

func New(numNodes int, outgoing chan packet.Message, timeout time.Duration, numRetries int) *Repeater {
	r := Repeater{
		outgoing:    outgoing,
		timeout:     timeout,
		numRetries:  numRetries,
		lock:        sync.Mutex{},
		unackedReqs: make(map[int]map[int]map[packet.Messagetype]bool),
	}

	for i := 0; i < numNodes; i++ {
		r.unackedReqs[i] = make(map[int]map[packet.Messagetype]bool)
	}

	return &r
}

func (r *Repeater) Send(msg packet.Message, unlimitedRepeats bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	var demuxKey packet.Messagetype

	switch msg.DemuxKey {
	case packet.NodeLockRequest:
		demuxKey = packet.NodeLockResponse
	case packet.NodeLockRequestNoTimeout:
		demuxKey = packet.NodeLockResponse
	case packet.NodeUnlockRequest:
		demuxKey = packet.NodeUnlockAck
	case packet.NodeGetRequest:
		demuxKey = packet.NodeGetResponse
	case packet.NodePutRequest:
		demuxKey = packet.NodePutResponse
	case packet.NodeTimestampRequest:
		demuxKey = packet.NodeGetResponse
	default:
		log.Fatal("Unexpected message type in Repeater.Send", msg)
	}

	if r.unackedReqs[msg.Dest][msg.Id] == nil {
		r.unackedReqs[msg.Dest][msg.Id] = make(map[packet.Messagetype]bool)
	}
	r.unackedReqs[msg.Dest][msg.Id][demuxKey] = true

	go r.send(msg, demuxKey, unlimitedRepeats)
}

func (r *Repeater) send(msg packet.Message, demuxKey packet.Messagetype, unlimited bool) {
	for i := 0; i < r.numRetries; i++ {
		r.lock.Lock()

		if !r.unackedReqs[msg.Dest][msg.Id][demuxKey] {
			r.lock.Unlock()
			return
		}

		r.outgoing <- msg

		r.lock.Unlock()

		time.Sleep(r.timeout)

		if unlimited {
			i--
		}
	}
}

func (r *Repeater) Ack(msg packet.Message) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.unackedReqs[msg.Src][msg.Id][msg.DemuxKey] {
		delete(r.unackedReqs[msg.Src][msg.Id], msg.DemuxKey)
	}
}
