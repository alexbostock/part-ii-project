package dbnode

import (
	"encoding/base64"
	"encoding/binary"
	"log"
	"math/rand"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexbostock/part-ii-project/datastore"
	"github.com/alexbostock/part-ii-project/dbnode/repeater"
	"github.com/alexbostock/part-ii-project/packet"
)

type mode int

const (
	idle mode = iota
	assemblingQuorum
	coordinatingRead
	coordinatingWrite
	processingRead
	processingWrite
)

type Dbnode struct {
	id              int
	numPeers        int
	readQuorumSize  int
	writeQuorumSize int
	Incoming        chan packet.Message
	Outgoing        chan packet.Message
	lockTimeout     time.Duration

	currentMode  mode
	Store        datastore.Store
	currentTxid  int
	lockRequests queue

	// State relevent in modes coordinatingRead and coordinatingWrite:

	clientRequest packet.Message
	quorumMembers map[int]packet.Message
	// The number of nodes we are waiting for before we can continue
	numWaitingNodes int

	requestRepeater *repeater.Repeater

	uncommitedTxid int
	uncommitedKey  []byte
}

func New(n int, id int, lockTimeout time.Duration) *Dbnode {
	outgoing := make(chan packet.Message, 1000)

	state := &Dbnode{
		id:              id,
		numPeers:        n - 1,
		readQuorumSize:  n/2 + 1,
		writeQuorumSize: n/2 + 1,
		Incoming:        make(chan packet.Message, 1000),
		Outgoing:        outgoing,
		lockTimeout:     lockTimeout,
		Store:           datastore.New(filepath.Join("data", strconv.Itoa(id))),
		currentTxid:     -1,

		// TODO: change 1 to a larger value to actually resend messages
		requestRepeater: repeater.New(n, outgoing, lockTimeout, 1),
	}

	go state.handleRequests()

	return state
}

func (n *Dbnode) handleRequests() {
	timeoutCounter := 0
	go n.setTimer(timeoutCounter)

	timedOutLockRequests := make(chan *packet.Message, 10)

	for {
		select {
		case msg := <-n.Incoming:
			if msg.Dest != n.id {
				log.Fatal("Midelivered message", msg)
			}

			if msg.DemuxKey == packet.InternalTimerSignal {
				if msg.Id == timeoutCounter {
					switch n.currentMode {
					case coordinatingRead:
						n.abortProcessing()
					case assemblingQuorum:
						n.abortProcessing()
					case processingRead:
						n.abortProcessing()
					}
				}
				go n.setTimer(timeoutCounter)
			} else {
				timeoutCounter++
			}

			switch msg.DemuxKey {
			case packet.ClientReadRequest:
				n.lockRequests.enqueue(&msg)
			case packet.ClientWriteRequest:
				n.lockRequests.enqueue(&msg)
			case packet.NodeLockRequest:
				fallthrough
			case packet.NodeLockRequestNoTimeout:
				n.lockRequests.enqueue(&msg)
				go func() {
					time.Sleep(n.lockTimeout)
					timedOutLockRequests <- &msg
				}()
			case packet.NodeLockResponse:
				n.handleLockRes(msg)
			case packet.NodeUnlockRequest:
				n.handleUnlockReq(msg)
			case packet.NodeUnlockAck:
				n.handleUnlockAck(msg)
			case packet.NodeGetRequest:
				n.handleGetReq(msg)
			case packet.NodeGetResponse:
				n.handleGetRes(msg)
			case packet.NodePutRequest:
				n.handlePutReq(msg)
			case packet.NodePutResponse:
				n.handlePutRes(msg)
			case packet.NodeTimestampRequest:
				n.handleTimestampReq(msg)
			case packet.InternalTimerSignal:
				// Do nothing (already dealt with above)
			default:
				log.Fatal("Unexpected message type", msg)
			}

			if n.currentMode == idle && !n.lockRequests.empty() {
				msg = *n.lockRequests.dequeue()

				n.currentTxid = msg.Id
				switch msg.DemuxKey {
				case packet.ClientReadRequest:
					n.currentMode = coordinatingRead
					n.clientRequest = msg
					n.continueProcessing()
				case packet.ClientWriteRequest:
					n.currentMode = assemblingQuorum
					n.clientRequest = msg
					n.continueProcessing()
				case packet.NodeLockRequest:
					n.currentMode = processingRead
					n.currentTxid = msg.Id

					n.Outgoing <- packet.Message{
						Id:       msg.Id,
						Src:      n.id,
						Dest:     msg.Src,
						DemuxKey: packet.NodeLockResponse,
						Ok:       true,
					}
				case packet.NodeLockRequestNoTimeout:
					n.currentMode = processingWrite
					n.currentTxid = msg.Id

					n.Outgoing <- packet.Message{
						Id:       msg.Id,
						Src:      n.id,
						Dest:     msg.Src,
						DemuxKey: packet.NodeLockResponse,
						Ok:       true,
					}
				default:
					log.Fatal("Unexpected message type", msg)
				}
			}
		case msg := <-timedOutLockRequests:
			if n.lockRequests.remove(msg) {
				var resType packet.Messagetype
				switch msg.DemuxKey {
				case packet.ClientReadRequest:
					resType = packet.ClientReadResponse
				case packet.ClientWriteRequest:
					resType = packet.ClientWriteResponse
				default:
					resType = packet.NodeLockResponse
				}

				n.Outgoing <- packet.Message{
					Id:       msg.Id,
					Src:      n.id,
					Dest:     msg.Src,
					DemuxKey: resType,
					Key:      msg.Key,
					Value:    msg.Value,
					Ok:       false,
				}
			}
		}
	}
}

func (n *Dbnode) setTimer(counter int) {
	time.Sleep(n.lockTimeout)
	n.Incoming <- packet.Message{
		Id:       counter,
		Src:      n.id,
		Dest:     n.id,
		DemuxKey: packet.InternalTimerSignal,
	}
}

func (n *Dbnode) handleLockRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && (n.currentMode == coordinatingRead || n.currentMode == assemblingQuorum) {
		if !msg.Ok {
			n.abortProcessing()
			return
		}

		// Make sure we don't count multiple response from the same node
		if n.quorumMembers[msg.Src].DemuxKey != msg.DemuxKey {
			n.quorumMembers[msg.Src] = msg
			n.numWaitingNodes--
			if n.numWaitingNodes == 0 {
				n.continueProcessing()
			}
		}
	} else if msg.Ok {
		n.requestRepeater.Send(packet.Message{
			Id:       msg.Id,
			Src:      n.id,
			Dest:     msg.Src,
			DemuxKey: packet.NodeUnlockRequest,
			Ok:       false,
		})
	}
}

func (n *Dbnode) handleUnlockReq(msg packet.Message) {
	if n.currentTxid == msg.Id && n.uncommitedTxid > 0 && msg.Ok {
		n.Store.Commit(n.uncommitedKey, n.uncommitedTxid)
		n.uncommitedKey = nil
		n.uncommitedTxid = 0
	}

	if n.currentTxid == msg.Id {
		n.currentMode = idle
		n.currentTxid = -1
	}

	n.Outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodeUnlockAck,
		Ok:       true,
	}
}

func (n *Dbnode) handleUnlockAck(msg packet.Message) {
	n.requestRepeater.Ack(msg)
}

func (n *Dbnode) handleGetReq(msg packet.Message) {
	var val []byte
	var ok bool

	if n.currentMode == processingRead && n.currentTxid == msg.Id {
		val = n.Store.Get(msg.Key)
		ok = val != nil
	}
	n.Outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodeGetResponse,
		Key:      msg.Key,
		Value:    val,
		Ok:       ok,
	}
}

func (n *Dbnode) handleGetRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && (n.currentMode == coordinatingRead || n.currentMode == coordinatingWrite) {
		if !msg.Ok {
			n.abortProcessing()
			return
		}

		if n.quorumMembers[msg.Src].DemuxKey != msg.DemuxKey {
			n.quorumMembers[msg.Src] = msg
			n.numWaitingNodes--
			if n.numWaitingNodes == 0 {
				n.continueProcessing()
			}
		}
	}
}

func (n *Dbnode) handlePutReq(msg packet.Message) {
	var ok bool

	if n.currentMode == processingWrite && n.currentTxid == msg.Id {
		n.uncommitedTxid = n.Store.Put(msg.Key, msg.Value)
		if n.uncommitedTxid > 0 {
			n.uncommitedKey = msg.Key
			ok = true
		}
	}

	n.Outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodePutResponse,
		Key:      msg.Key,
		Value:    msg.Value,
		Ok:       ok,
	}
}

func (n *Dbnode) handlePutRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && n.currentMode == coordinatingWrite {
		if !msg.Ok {
			n.abortProcessing()
			return
		}

		if n.quorumMembers[msg.Src].DemuxKey != msg.DemuxKey {
			n.quorumMembers[msg.Src] = msg
			n.numWaitingNodes--
			if n.numWaitingNodes == 0 {
				n.continueProcessing()
			}
		}
	}
}

func (n *Dbnode) handleTimestampReq(msg packet.Message) {
	// Must always respond with nodeGetResponse, ok: true

	var val []byte

	if n.currentMode == processingWrite && n.currentTxid == msg.Id {
		val = n.Store.Get(msg.Key)
	}

	if len(val) == 0 {
		// Respond with 0 timestamp on failure
		val = make([]byte, 12)
		var ts uint64 = 0
		tsBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(tsBytes, ts)
		base64.StdEncoding.Encode(val, tsBytes)
	}

	n.Outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodeGetResponse,
		Key:      msg.Key,
		Value:    val,
		Ok:       true,
	}
}

func (n *Dbnode) continueProcessing() {
	switch n.currentMode {
	case coordinatingRead:
		if n.quorumMembers == nil {
			n.assembleQuorum(n.readQuorumSize, packet.NodeLockRequest)

			return
		}
		switch n.quorumMembers[n.id].DemuxKey {
		case packet.NodeGetRequest:
			for node := range n.quorumMembers {
				if node == n.id {
					continue
				}

				n.requestRepeater.Send(packet.Message{
					Id:       n.currentTxid,
					Src:      n.id,
					Dest:     node,
					DemuxKey: packet.NodeGetRequest,
					Key:      n.clientRequest.Key,
					Ok:       true,
				})
			}
		case packet.NodeUnlockRequest:
			// Read local value
			var timestamp uint64
			var value []byte

			localVal := n.Store.Get(n.clientRequest.Key)
			if len(localVal) > 0 {
				timestamp, value = decodeTimestampVal(localVal)
			} else {
				timestamp = 0
			}

			// Find the most recent value

			for id, res := range n.quorumMembers {
				if id == n.id {
					continue
				}

				if len(res.Value) > 0 {
					t, v := decodeTimestampVal(res.Value)
					if t > timestamp {
						timestamp = t
						value = v
					}
				}

				// Unlock each node
				n.requestRepeater.Send(packet.Message{
					Id:       res.Id,
					Src:      n.id,
					Dest:     id,
					DemuxKey: packet.NodeUnlockRequest,
					Ok:       true,
				})
			}

			// Return to client
			n.Outgoing <- packet.Message{
				Id:       n.clientRequest.Id,
				Src:      n.id,
				Dest:     n.clientRequest.Src,
				DemuxKey: packet.ClientReadResponse,
				Key:      n.clientRequest.Key,
				Value:    value,
				Ok:       true,
			}

			// Return to idle state
			n.currentMode = idle
			n.currentTxid = -1
			n.quorumMembers = nil
			n.numWaitingNodes = 0
		default:
			log.Fatal("Read coordinator has reached an invalid state", n.quorumMembers[n.id])
		}
	case coordinatingWrite:
		switch n.quorumMembers[n.id].DemuxKey {
		case packet.NodeTimestampRequest:
			for node := range n.quorumMembers {
				if node == n.id {
					continue
				}

				n.requestRepeater.Send(packet.Message{
					Id:       n.clientRequest.Id,
					Src:      n.id,
					Dest:     node,
					DemuxKey: packet.NodeTimestampRequest,
					Key:      n.clientRequest.Key,
					Ok:       true,
				})
			}

			n.quorumMembers[n.id] = packet.Message{
				DemuxKey: packet.NodePutRequest,
			}
			n.numWaitingNodes = n.writeQuorumSize - 1
		case packet.NodePutRequest:
			var latestTimestamp uint64

			localVal := n.Store.Get(n.clientRequest.Key)
			if len(localVal) > 0 {
				return
				latestTimestamp, _ = decodeTimestampVal(localVal)
			}

			for id, msg := range n.quorumMembers {
				if id == n.id {
					continue
				}

				timestamp, _ := decodeTimestampVal(msg.Value)
				if timestamp > latestTimestamp {
					latestTimestamp = timestamp
				}
			}

			timestampBytes := make([]byte, 8)
			value := make([]byte, 12)
			binary.BigEndian.PutUint64(timestampBytes, latestTimestamp+1)
			base64.StdEncoding.Encode(value, timestampBytes)

			value = append(value, n.clientRequest.Value...)

			n.uncommitedTxid = n.Store.Put(n.clientRequest.Key, value)
			n.uncommitedKey = n.clientRequest.Key

			for id := range n.quorumMembers {
				if id == n.id {
					continue
				}

				n.requestRepeater.Send(packet.Message{
					Id:       n.clientRequest.Id,
					Src:      n.id,
					Dest:     id,
					DemuxKey: packet.NodePutRequest,
					Key:      n.clientRequest.Key,
					Value:    value,
					Ok:       true,
				})
			}

			n.quorumMembers[n.id] = packet.Message{
				DemuxKey: packet.NodeUnlockRequest,
			}
			n.numWaitingNodes = n.writeQuorumSize - 1
		case packet.NodeUnlockRequest:
			ok := n.Store.Commit(n.uncommitedKey, n.uncommitedTxid)

			n.Outgoing <- packet.Message{
				Id:       n.clientRequest.Id,
				Src:      n.id,
				Dest:     n.clientRequest.Src,
				DemuxKey: packet.ClientWriteResponse,
				Key:      n.clientRequest.Key,
				Value:    n.clientRequest.Value,
				Ok:       ok,
			}

			n.uncommitedKey = nil
			n.uncommitedTxid = 0

			for id := range n.quorumMembers {
				if id == n.id {
					continue
				}

				n.requestRepeater.Send(packet.Message{
					Id:       n.clientRequest.Id,
					Src:      n.id,
					Dest:     id,
					DemuxKey: packet.NodeUnlockRequest,
					Ok:       true,
				})
			}

			n.currentMode = idle
			n.currentTxid = -1
			n.quorumMembers = nil
			n.numWaitingNodes = 0
		}
	case assemblingQuorum:
		if n.quorumMembers == nil {
			n.assembleQuorum(n.writeQuorumSize, packet.NodeLockRequestNoTimeout)
		} else {
			n.currentMode = coordinatingWrite
			n.continueProcessing()
		}
	}
}

func (n *Dbnode) abortProcessing() {
	// TODO: rollback any uncommitted transaction

	if n.quorumMembers != nil {
		for node := range n.quorumMembers {
			if node == n.id {
				continue
			}

			n.requestRepeater.Send(packet.Message{
				Id:       n.clientRequest.Id,
				Src:      n.id,
				Dest:     node,
				DemuxKey: packet.NodeUnlockRequest,
				Ok:       false,
			})
		}
	}

	switch n.currentMode {
	case assemblingQuorum:
		fallthrough
	case coordinatingRead:
		fallthrough
	case coordinatingWrite:
		var resType packet.Messagetype
		if n.currentMode == coordinatingRead {
			resType = packet.ClientReadResponse
		} else {
			resType = packet.ClientWriteResponse
		}

		n.Outgoing <- packet.Message{
			Id:       n.clientRequest.Id,
			Src:      n.id,
			Dest:     n.clientRequest.Src,
			DemuxKey: resType,
			Key:      n.clientRequest.Key,
			Value:    n.clientRequest.Value,
			Ok:       false,
		}
	case processingRead:
		fallthrough
	case processingWrite:
		n.Outgoing <- packet.Message{
			Id:       n.clientRequest.Id,
			Src:      n.id,
			Dest:     n.clientRequest.Src,
			DemuxKey: packet.NodeUnlockAck,
			Ok:       false,
		}
	}

	n.currentMode = idle
	n.currentTxid = -1
	n.quorumMembers = nil
	n.numWaitingNodes = 0
}

func (n *Dbnode) assembleQuorum(quorumSize int, requestType packet.Messagetype) {
	n.quorumMembers = make(map[int]packet.Message)

	peers := rand.Perm(n.numPeers)
	for _, node := range peers[:quorumSize-1] {
		if node == n.id {
			node = n.numPeers
		}
		n.quorumMembers[node] = packet.Message{
			Id:       n.clientRequest.Id,
			Src:      n.id,
			Dest:     node,
			DemuxKey: requestType,
			Ok:       true,
		}
		n.requestRepeater.Send(n.quorumMembers[node])
	}

	// n.quorumMembers[n.id] is a marker of the next step
	if requestType == packet.NodeLockRequest {
		n.quorumMembers[n.id] = packet.Message{
			DemuxKey: packet.NodeGetRequest,
		}
	} else if requestType == packet.NodeLockRequestNoTimeout {
		n.quorumMembers[n.id] = packet.Message{
			DemuxKey: packet.NodeTimestampRequest,
		}
	}

	n.numWaitingNodes = quorumSize - 1
}
