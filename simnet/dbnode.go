package simnet

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexbostock/part-ii-project/simnet/datastore"
	"github.com/alexbostock/part-ii-project/simnet/packet"
	"github.com/alexbostock/part-ii-project/simnet/repeater"
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

type nodestate struct {
	id              int
	numPeers        int
	readQuorumSize  int
	writeQuorumSize int
	outgoing        chan packet.Message
	lockTimeout     time.Duration

	currentMode  mode
	store        datastore.Store
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

func startNode(n int, id int, incoming, outgoing chan packet.Message, lockTimeout time.Duration) {
	state := &nodestate{
		id:              id,
		numPeers:        n - 1,
		readQuorumSize:  n/2 + 1,
		writeQuorumSize: n/2 + 1,
		outgoing:        outgoing,
		lockTimeout:     lockTimeout,
		store:           datastore.New(filepath.Join("data", strconv.Itoa(id))),
		currentTxid:     -1,

		requestRepeater: repeater.New(n, outgoing, lockTimeout, 3),
	}

	state.handleRequests(incoming)
}

func (n *nodestate) handleRequests(incoming chan packet.Message) {
	timeoutCounter := 0
	go n.setTimer(incoming, timeoutCounter)

	timedOutLockRequests := make(chan *packet.Message, 10)

	for {
		select {
		case msg := <-incoming:
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
				go n.setTimer(incoming, timeoutCounter)
			} else {
				timeoutCounter++
				fmt.Printf("%+v\n", n)
				msg.Print()
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

					n.outgoing <- packet.Message{
						Id:       msg.Id,
						Src:      n.id,
						Dest:     msg.Src,
						DemuxKey: packet.NodeLockResponse,
						Ok:       true,
					}
				case packet.NodeLockRequestNoTimeout:
					n.currentMode = processingWrite
					n.currentTxid = msg.Id

					n.outgoing <- packet.Message{
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
			if msg.DemuxKey != packet.InternalTimerSignal {
				fmt.Printf("%+v\n\n", n)
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
					resType = packet.NodeGetResponse
				}

				n.outgoing <- packet.Message{
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

func (n *nodestate) setTimer(incoming chan packet.Message, counter int) {
	time.Sleep(n.lockTimeout)
	incoming <- packet.Message{
		Id:       counter,
		Src:      n.id,
		Dest:     n.id,
		DemuxKey: packet.InternalTimerSignal,
	}
}

func (n *nodestate) handleLockRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && (n.currentMode == coordinatingRead || n.currentMode == assemblingQuorum) {
		if !msg.Ok {
			n.abortProcessing()
			return
		}
		n.quorumMembers[msg.Src] = msg
		n.numWaitingNodes--
		if n.numWaitingNodes == 0 {
			n.continueProcessing()
		}
	}
}

func (n *nodestate) handleUnlockReq(msg packet.Message) {
	if n.currentTxid == msg.Id && n.uncommitedTxid > 0 && msg.Ok {
		n.store.Commit(n.uncommitedKey, n.uncommitedTxid)
		n.uncommitedKey = nil
		n.uncommitedTxid = 0
	}

	n.currentMode = idle
	n.currentTxid = -1

	n.outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodeUnlockAck,
		Ok:       true,
	}
}

func (n *nodestate) handleUnlockAck(msg packet.Message) {
	n.requestRepeater.Ack(msg)
}

func (n *nodestate) handleGetReq(msg packet.Message) {
	var val []byte
	var ok bool

	if n.currentMode == processingRead && n.currentTxid == msg.Id {
		val = n.store.Get(msg.Key)
		ok = val != nil
	}
	n.outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodeGetResponse,
		Key:      msg.Key,
		Value:    val,
		Ok:       ok,
	}
}

func (n *nodestate) handleGetRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && (n.currentMode == coordinatingRead || n.currentMode == coordinatingWrite) {
		if !msg.Ok {
			n.abortProcessing()
			return
		}
		n.quorumMembers[msg.Src] = msg
		n.numWaitingNodes--
		if n.numWaitingNodes == 0 {
			n.continueProcessing()
		}
	}
}

func (n *nodestate) handlePutReq(msg packet.Message) {
	var ok bool

	if n.currentMode == processingWrite && n.currentTxid == msg.Id {
		n.uncommitedTxid = n.store.Put(msg.Key, msg.Value)
		if n.uncommitedTxid > 0 {
			n.uncommitedKey = msg.Key
			ok = true
		}
	}

	n.outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodePutResponse,
		Key:      msg.Key,
		Value:    msg.Value,
		Ok:       ok,
	}
}

func (n *nodestate) handlePutRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && n.currentMode == coordinatingWrite {
		if !msg.Ok {
			n.abortProcessing()
			return
		}
		n.quorumMembers[msg.Src] = msg
		n.numWaitingNodes--
		if n.numWaitingNodes == 0 {
			n.continueProcessing()
		}
	}
}

func (n *nodestate) handleTimestampReq(msg packet.Message) {
	// Must always respond with nodeGetResponse, ok: true

	var val []byte

	if n.currentMode == processingWrite && n.currentTxid == msg.Id {
		val = n.store.Get(msg.Key)
	}

	if len(val) == 0 {
		// Respond with 0 timestamp on failure
		val = make([]byte, 12)
		var ts uint64 = 0
		tsBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(tsBytes, ts)
		base64.StdEncoding.Encode(val, tsBytes)
	}

	n.outgoing <- packet.Message{
		Id:       msg.Id,
		Src:      n.id,
		Dest:     msg.Src,
		DemuxKey: packet.NodeGetResponse,
		Key:      msg.Key,
		Value:    val,
		Ok:       true,
	}
}

func (n *nodestate) continueProcessing() {
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

			localVal := n.store.Get(n.clientRequest.Key)
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
			n.outgoing <- packet.Message{
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

			localVal := n.store.Get(n.clientRequest.Key)
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

			n.uncommitedTxid = n.store.Put(n.clientRequest.Key, value)
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
			ok := n.store.Commit(n.uncommitedKey, n.uncommitedTxid)

			n.outgoing <- packet.Message{
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

func (n *nodestate) abortProcessing() {
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

		n.outgoing <- packet.Message{
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
		n.outgoing <- packet.Message{
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

func (n *nodestate) assembleQuorum(quorumSize int, requestType packet.Messagetype) {
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
