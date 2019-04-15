package elector

import (
	"time"

	"github.com/alexbostock/part-ii-project/net/packet"
)

// A bully is an Elector based on the bully algorithm.
type bully struct {
	id       int
	n        int
	timeout  time.Duration
	outgoing chan packet.Message

	// leader == id => this node is leader.
	// leader == -1 => election in progress.
	leader      int
	maybeLeader bool

	messageQueue       chan packet.Message
	leaderQueryResChan chan int
	requestsToForward  chan packet.Message

	disabled bool

	internalTimer chan int
}

func newBully(id, n int, outgoing chan packet.Message) *bully {
	b := &bully{
		id:       id,
		n:        n,
		timeout:  50 * time.Millisecond,
		outgoing: outgoing,

		leader: n - 1,

		messageQueue:       make(chan packet.Message, 10),
		leaderQueryResChan: make(chan int),
		requestsToForward:  make(chan packet.Message, 100),

		internalTimer: make(chan int),
	}

	go b.mainLoop()
	go b.startHeartbeat()

	return b
}

func (b *bully) mainLoop() {
	go b.startInternalTimer()

	timeoutCounter := 0
	b.internalTimer <- timeoutCounter

	for msg := range b.messageQueue {
		if b.disabled || msg.DemuxKey == packet.ControlRecover {
			if b.disabled && msg.DemuxKey == packet.ControlRecover {
				b.disabled = false
				timeoutCounter = 0
				b.internalTimer <- timeoutCounter

				b.startElection()
			}

			continue
		}

		if msg.DemuxKey == packet.ControlFail {
			b.disabled = true
			continue
		}

		if msg.DemuxKey == packet.InternalTimerSignal {
			if msg.Id == timeoutCounter {
				if b.leader == -1 && b.maybeLeader {
					b.becomeCoordinator()
				} else {
					b.startElection()
				}
			}
			b.internalTimer <- timeoutCounter
		} else {
			timeoutCounter++
		}

		switch msg.DemuxKey {
		case packet.ElectionElect:
			b.outgoing <- packet.Message{
				Src:      b.id,
				Dest:     msg.Src,
				DemuxKey: packet.ElectionAck,
				Ok:       true,
			}

			b.startElection()
		case packet.ElectionAck:
			b.maybeLeader = false
		case packet.ElectionCoordinator:
			if b.leader == msg.Src || b.leader == -1 {
				b.leader = msg.Src
				b.forwardRequests()
			} else {
				b.startElection()
			}
		case packet.InternalHeartbeat:
			if b.leader == b.id {
				b.broadcastHeartbeat()
			}
		case packet.InternalLeaderQuery:
			b.leaderQueryResChan <- b.leader
		}
	}
}

func (b *bully) startInternalTimer() {
	for {
		c := <-b.internalTimer
		time.Sleep(b.timeout)
		b.messageQueue <- packet.Message{
			Id:       c,
			DemuxKey: packet.InternalTimerSignal,
		}
	}
}

func (b *bully) startHeartbeat() {
	for {
		time.Sleep(b.timeout * 2 / 5)
		b.messageQueue <- packet.Message{
			DemuxKey: packet.InternalHeartbeat,
		}
	}
}

func (b *bully) becomeCoordinator() {
	b.leader = b.id

	for i := 0; i < b.n; i++ {
		if i == b.id {
			continue
		}

		b.outgoing <- packet.Message{
			Src:      b.id,
			Dest:     i,
			DemuxKey: packet.ElectionCoordinator,
			Ok:       true,
		}
	}

}

func (b *bully) startElection() {
	b.maybeLeader = true
	b.leader = -1

	for i := b.id + 1; i < b.n; i++ {
		b.outgoing <- packet.Message{
			Src:      b.id,
			Dest:     i,
			DemuxKey: packet.ElectionElect,
			Ok:       true,
		}
	}

	if b.id == b.n-1 {
		b.becomeCoordinator()
	}
}

func (b *bully) broadcastHeartbeat() {
	for i := 0; i < b.n; i++ {
		if i == b.id {
			continue
		}

		b.outgoing <- packet.Message{
			Src:      b.id,
			Dest:     i,
			DemuxKey: packet.ElectionCoordinator,
			Ok:       true,
		}
	}
}

func (b *bully) forwardRequests() {
	for len(b.requestsToForward) > 0 {
		msg := <-b.requestsToForward

		b.outgoing <- packet.Message{
			Id:        msg.Id,
			Src:       msg.Src,
			Dest:      b.leader,
			DemuxKey:  msg.DemuxKey,
			Key:       msg.Key,
			Value:     msg.Value,
			Timestamp: msg.Timestamp,
			Ok:        msg.Ok,
		}
	}
}

// Leader returns the current leader, or -1 if an election is in progress.
func (b *bully) Leader() int {
	b.messageQueue <- packet.Message{
		DemuxKey: packet.InternalLeaderQuery,
	}

	return <-b.leaderQueryResChan
}

// ProcessMsg is a receiver for packets. It should be sent all Election messages
// received by a Dbnode.
func (b *bully) ProcessMsg(msg packet.Message) {
	b.messageQueue <- msg
}

// ForwardToLeader forwards a message to the current leader. If an election is
// in progress, it buffers the message and forwards it once a leader has been
// elected.
func (b *bully) ForwardToLeader(msg packet.Message) {
	b.requestsToForward <- msg
}
