package elector

import (
	"log"
	"strconv"
	"time"

	"github.com/alexbostock/part-ii-project/net/packet"
)

// A ring is an Elector based on a ring-based algorithm.
type ring struct {
	id       int
	n        int
	timeout  time.Duration
	outgoing chan packet.Message

	leader     int
	nextInRing int
	token      packet.Message

	messageQueue       chan packet.Message
	leaderQueryResChan chan int
	requestsToForward  chan packet.Message

	disabled bool

	internalTimer chan int

	tokenSentLast time.Time
}

func newRing(id, n int, outgoing chan packet.Message) *ring {
	r := &ring{
		id:       id,
		n:        n,
		timeout:  50 * time.Millisecond,
		outgoing: outgoing,

		leader:     -1,
		nextInRing: id + 1,

		messageQueue:       make(chan packet.Message, n),
		leaderQueryResChan: make(chan int),
		requestsToForward:  make(chan packet.Message, 1000),

		internalTimer: make(chan int),
	}

	if r.nextInRing == n {
		r.nextInRing = 0
	}

	r.token = packet.Message{
		Id:       r.leader,
		Src:      r.id,
		Dest:     r.nextInRing,
		DemuxKey: packet.ElectionElect,
		Ok:       true,
	}

	go r.mainLoop()

	return r
}

func (r *ring) mainLoop() {
	go r.startInternalTimer()

	timeoutCounter := 0
	r.internalTimer <- timeoutCounter

	if r.id == r.n-1 {
		r.token.Value = addId(r.token.Value, r.id)
		r.forwardToken()
	}

	for msg := range r.messageQueue {
		if r.disabled || msg.DemuxKey == packet.ControlRecover {
			if r.disabled && msg.DemuxKey == packet.ControlRecover {
				r.disabled = false
				timeoutCounter = 0
				r.internalTimer <- timeoutCounter
			}

			continue
		}

		if msg.DemuxKey == packet.ControlFail {
			r.disabled = true
			r.leader = -1
			continue
		}

		if msg.DemuxKey == packet.InternalTimerSignal {
			if msg.Id == timeoutCounter {
				if r.leader == r.nextInRing-1 {
					r.token.Value = removeId(r.token.Value, r.leader)
					r.leader = highestId(r.token.Value)
				}
				r.token.Src = r.id
				r.token.Dest = r.nextInRing
				r.forwardToken()
				r.nextInRing++
				if r.nextInRing == r.n {
					r.nextInRing = 0
				}
			}
			r.internalTimer <- timeoutCounter
		} else {
			timeoutCounter++
		}

		switch msg.DemuxKey {
		case packet.ElectionElect:
			r.outgoing <- packet.Message{
				Src:      r.id,
				Dest:     msg.Src,
				DemuxKey: packet.ElectionAck,
				Ok:       true,
			}

			r.token = msg
			r.token.Value = addId(r.token.Value, r.id)
			r.leader = highestId(r.token.Value)

			r.token.Src = r.id
			r.token.Dest = r.nextInRing
			r.forwardToken()

			r.nextInRing = r.id + 1
			if r.nextInRing == r.n {
				r.nextInRing = 0
			}
		case packet.ElectionAck:
			continue
		case packet.InternalLeaderQuery:
			r.leaderQueryResChan <- r.leader
		}

		if r.leader > -1 {
			r.forwardRequests()
		}
	}
}

func (r *ring) startInternalTimer() {
	for {
		c := <-r.internalTimer
		time.Sleep(r.timeout / 5)
		r.messageQueue <- packet.Message{
			Id:       c,
			DemuxKey: packet.InternalTimerSignal,
		}
	}
}

func (r *ring) forwardRequests() {
	for len(r.requestsToForward) > 0 {
		msg := <-r.requestsToForward

		r.outgoing <- packet.Message{
			Id:        msg.Id,
			Src:       msg.Src,
			Dest:      r.leader,
			DemuxKey:  msg.DemuxKey,
			Key:       msg.Key,
			Value:     msg.Value,
			Timestamp: msg.Timestamp,
			Ok:        msg.Ok,
		}
	}
}

func (r *ring) forwardToken() {
	if time.Since(r.tokenSentLast) > r.timeout/5 {
		r.outgoing <- r.token
		r.tokenSentLast = time.Now()
	}
}

func (r *ring) Leader() int {
	r.messageQueue <- packet.Message{
		DemuxKey: packet.InternalLeaderQuery,
	}

	return <-r.leaderQueryResChan
}

func (r *ring) ProcessMsg(msg packet.Message) {
	r.messageQueue <- msg
}

func (r *ring) ForwardToLeader(msg packet.Message) {
	r.requestsToForward <- msg
}

func containsId(b []byte, id int) bool {
	i := 0
	j := 0

	for i < len(b) {
		for j < len(b) && b[j] != 0 {
			j++
		}

		if bytesAsId(b[i:j]) == id {
			return true
		}

		i = j + 1
	}

	return false
}

func addId(b []byte, id int) []byte {
	end := idAsBytes(id)

	return append(b, end...)
}

func removeId(b []byte, id int) []byte {
	for i := 0; i < len(b); i += 3 {
		if bytesAsId(b[i:i+3]) == id {
			return append(b[:i], b[i+3:]...)
		}
	}

	return b
}

func highestId(b []byte) int {
	if len(b)%3 > 0 {
		log.Fatal("Invalid id ", b)
	}

	max := -1

	for i := 0; i < len(b); i += 3 {
		id := bytesAsId(b[i : i+3])
		if id > max {
			max = id
		}
	}

	return max
}

func idAsBytes(id int) []byte {
	b := []byte(strconv.Itoa(id))

	switch len(b) {
	case 1:
		return append([]byte("00"), b...)
	case 2:
		return append([]byte("0"), b...)
	case 3:
		return b
	default:
		log.Fatal("Id too big ", id)
	}

	return nil
}

func bytesAsId(b []byte) int {
	id, err := strconv.Atoi(string(b))
	if err != nil {
		log.Fatal("Failed to parse id ", b, err)
	}

	return id
}
