// Package repeater provides a service to automatically resend requests until
// they are acknowledged, optionally up to a maximum number of attempts.
package repeater

import (
	"log"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/packet"
)

// A Repeater provides methods to resend messages until they are acknowledged.
// All expected acknowledgement message types are hardcoded based on the quorum
// assembly and 2PC procedures. Repeater should be instantiated using New.
// There should be 1 Repeater per dbnode.Dbnode.
type Repeater struct {
	outgoing   chan packet.Message
	timeout    time.Duration
	numRetries int
	lock       sync.Mutex

	// nodeID -> txID -> messageType -> bool
	unackedReqs map[int]map[int]map[packet.Messagetype]bool

	disabled bool
}

// New creates an instance of Repeater. numNodes is the total number of
// database nodes. outgoing is the Outgoing link for the calling node. timeout
// is the delay between sends. numRetries is the maximum number of resends
// (except for messages with an unlimited number of resends).
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

// Send sends the given message, resending periodically until acknowledged
// or the max number of retries is reached. If unlimitedRepeats, keep resending
// until an acknowledgement is received.
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

		if !r.disabled {
			if !r.unackedReqs[msg.Dest][msg.Id][demuxKey] {
				r.lock.Unlock()
				return
			}

			r.outgoing <- msg
		}

		r.lock.Unlock()

		if unlimited {
			i--
		}

		if !r.disabled || unlimited {
			time.Sleep(r.timeout)
		}
	}
}

// Ack is used to acknowledge a message. Repeater does not monitor Incoming, so
// it needs to receive a copy of each message explicitly through Ack. The
// caller need not worry about matching requests to acknowledgements. Calling
// Ack more than once relating to the same initial message is fine.
func (r *Repeater) Ack(msg packet.Message) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.unackedReqs[msg.Src][msg.Id][msg.DemuxKey] {
		delete(r.unackedReqs[msg.Src][msg.Id], msg.DemuxKey)
	}
}

// Fail makes the module stop sending messages, to simulate node failure. This
// also cancels resending of any message without unlimitedRepeats.
func (r *Repeater) Fail() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.disabled = true
}

// Recover makes the module resume from a failure.
func (r *Repeater) Recover() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.disabled = false
}
