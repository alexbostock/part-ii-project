// Package dbnode implements the behaviour of each node in a distributed
// database based on quorum assembly.
package dbnode

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexbostock/part-ii-project/datastore"
	"github.com/alexbostock/part-ii-project/dbnode/elector"
	"github.com/alexbostock/part-ii-project/dbnode/repeater"
	"github.com/alexbostock/part-ii-project/net/packet"
)

type mode int

const (
	idle mode = iota
	assemblingQuorum
	coordinatingRead
	coordinatingWrite
	processingRead
	processingWrite
	coordinatingFastRead
)

const fastReads = true

// A Dbnode is a single database node. In order to behave like a node, it
// should be instantiated with New. Public fields are Incoming and Outgoing
// simulated network links and Store, the underlying local datastore.
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

	requestRepeater       *repeater.Repeater
	backgroundWriteDaemon *propagater

	uncommitedTxid int
	uncommitedKey  []byte

	// IDs from previous unlock transactions to guard against case of
	// unlock received before corresponding lock.
	unlockTxids map[int]bool

	internalTimer chan int
	elector       elector.Elector

	disabled bool

	stateQueryReq chan bool
	stateQueryRes chan int

	logWrites bool
}

// New creates a new database node and starts the main loop to handle requests
// from Incoming. The main loop runs in a separate goroutine, so this method
// without delay.
//
// Parameters:
// n: the number of database nodes in the system.
// id: the id of this node (0 <= id < n).
// lockTimeout: the time to wait before aborting a transaction (where applicable).
// persistentStore: indicates whether the underlying store should use disk or
// main memory.
// rqs: the minimum size of a read quorum.
// wqs: the minimum size of a write quorum.
// sloppyQuorum: true enables background writes to achieve eventual consistency.
func New(n int, id int, lockTimeout time.Duration, persistentStore bool, rqs uint, wqs uint, sloppyQuorum bool, logWrites bool) *Dbnode {
	outgoing := make(chan packet.Message, 1000)

	var store datastore.Store
	if persistentStore {
		store = datastore.New(filepath.Join("data", strconv.Itoa(id)))
	} else {
		store = datastore.New("")
	}

	var p *propagater
	if sloppyQuorum {
		p = newPropagater(id, n, int(rqs), outgoing)
	}

	state := &Dbnode{
		id:              id,
		numPeers:        n - 1,
		readQuorumSize:  int(rqs),
		writeQuorumSize: int(wqs),
		Incoming:        make(chan packet.Message, 1000),
		Outgoing:        outgoing,
		lockTimeout:     lockTimeout,
		Store:           store,
		currentTxid:     -1,

		requestRepeater:       repeater.New(n, outgoing, lockTimeout, 3),
		backgroundWriteDaemon: p,
		unlockTxids:           make(map[int]bool),

		stateQueryReq: make(chan bool),
		stateQueryRes: make(chan int),

		internalTimer: make(chan int),
		elector:       elector.New(id, n, outgoing),

		logWrites: logWrites,
	}

	go state.handleRequests()

	return state
}

// The main loop. Only this method may access any node state. This goroutine
// must not block; all blocking operations should be in separate goroutines,
// which communicate with the main loop by sending messages.
func (n *Dbnode) handleRequests() {
	go n.setInternalTimer()

	timeoutCounter := 0
	n.internalTimer <- timeoutCounter

	timedOutLockRequests := make(chan *packet.Message, 10)

	for {
		select {
		case msg := <-n.Incoming:
			if n.disabled || msg.DemuxKey == packet.ControlRecover {
				if n.disabled && msg.DemuxKey == packet.ControlRecover {
					n.disabled = false
					timeoutCounter = 0
					n.internalTimer <- timeoutCounter

					n.requestRepeater.Recover()
					n.elector.ProcessMsg(packet.Message{
						DemuxKey: packet.ControlRecover,
					})

					fmt.Printf("Node %v recovered\n", n.id)
				}

				continue
			}

			if msg.DemuxKey == packet.ControlFail {
				if !n.disabled {
					n.disabled = true

					n.requestRepeater.Fail()
					n.elector.ProcessMsg(packet.Message{
						DemuxKey: packet.ControlFail,
					})

					fmt.Printf("Node %v failed while in mode %v\n", n.id, n.currentMode)

					if n.currentMode != coordinatingWrite && n.currentMode != assemblingQuorum {
						n.currentMode = idle
						n.currentTxid = -1
						n.clientRequest = packet.Message{}
						n.quorumMembers = nil
					}
				}

				continue
			}

			if msg.Dest != n.id {
				log.Fatal("Midelivered message", msg)
			}

			if msg.DemuxKey == packet.InternalTimerSignal {
				if msg.Id == timeoutCounter {
					switch n.currentMode {
					case coordinatingRead:
						n.abortProcessing()
					case coordinatingWrite:
						n.abortProcessing()
					case assemblingQuorum:
						n.abortProcessing()
					case processingRead:
						n.abortProcessing()
					}
				}
				n.internalTimer <- timeoutCounter
			} else {
				timeoutCounter++
			}

			// Occasionally delete stale unlockTxids
			if msg.DemuxKey == packet.ElectionElect {
				n.cleanUnlockTxids()
			}

			switch msg.DemuxKey {
			case packet.ClientWriteRequest, packet.ClientStrongWriteRequest:
				if n.writeQuorumSize == 1 {
					n.processLocalWrite(msg)
				} else if n.elector.Leader() == n.id {
					n.lockRequests.enqueue(&msg)
					go func() {
						time.Sleep(10 * n.lockTimeout)
						timedOutLockRequests <- &msg
					}()
				} else {
					n.elector.ForwardToLeader(msg)
				}
			case packet.ClientReadRequest, packet.NodeLockRequest, packet.NodeLockRequestNoTimeout:
				if msg.DemuxKey == packet.ClientReadRequest && n.readQuorumSize == 1 {
					n.processLocalRead(msg)
				} else {
					n.lockRequests.enqueue(&msg)
					go func() {
						time.Sleep(n.lockTimeout)
						timedOutLockRequests <- &msg
					}()
				}
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
			case packet.NodeBackgroundWriteRequest:
				n.handleBackgroundWriteReq(msg)
			case packet.NodeBackgroundWriteResponse:
				n.handleBackgroundWriteRes(msg)
			case packet.InternalTimerSignal:
				// Do nothing (already dealt with above)
			case packet.ElectionElect, packet.ElectionCoordinator, packet.ElectionAck:
				timeoutCounter--
				n.elector.ProcessMsg(msg)
			default:
				log.Fatal("Unexpected message type", msg)
			}

			if n.currentMode == idle && !n.lockRequests.empty() {
				msg = *n.lockRequests.dequeue()

				// Don't lock for a transaction for which we have
				// previously received an unlock request.
				if n.unlockTxids[msg.Id] {
					continue
				}

				n.currentTxid = msg.Id
				n.clientRequest = msg

				switch msg.DemuxKey {
				case packet.ClientReadRequest:
					if fastReads {
						n.currentMode = coordinatingFastRead
					} else {
						n.currentMode = coordinatingRead
					}
					n.continueProcessing()
				case packet.ClientWriteRequest, packet.ClientStrongWriteRequest:
					n.currentMode = assemblingQuorum
					n.continueProcessing()
				case packet.NodeLockRequest:
					n.currentMode = processingRead
					n.Outgoing <- packet.Message{
						Id:       msg.Id,
						Src:      n.id,
						Dest:     msg.Src,
						DemuxKey: packet.NodeLockResponse,
						Ok:       true,
					}
				case packet.NodeLockRequestNoTimeout:
					n.currentMode = processingWrite
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
			if n.disabled {
				continue
			}

			if n.lockRequests.remove(msg) {
				var resType packet.Messagetype
				switch msg.DemuxKey {
				case packet.ClientReadRequest:
					resType = packet.ClientReadResponse
				case packet.ClientWriteRequest, packet.ClientStrongWriteRequest:
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
			} else if msg.Id == n.currentTxid && (msg.DemuxKey == packet.ClientReadRequest || msg.DemuxKey == packet.ClientWriteRequest || msg.DemuxKey == packet.ClientStrongWriteRequest) {
				n.abortProcessing()
			}
		case <-n.stateQueryReq:
			n.stateQueryRes <- n.currentTxid
		}
	}
}

func (n *Dbnode) setInternalTimer() {
	for {
		c := <-n.internalTimer
		time.Sleep(n.lockTimeout)
		n.Incoming <- packet.Message{
			Id:       c,
			Src:      n.id,
			Dest:     n.id,
			DemuxKey: packet.InternalTimerSignal,
		}
	}
}

func (n *Dbnode) cleanUnlockTxids() {
	var threshold int
	for id := range n.unlockTxids {
		if id > threshold {
			threshold = id
		}
	}

	threshold -= 50

	for id := range n.unlockTxids {
		if id < threshold {
			delete(n.unlockTxids, id)
		}
	}
}

func (n *Dbnode) processLocalRead(msg packet.Message) {
	// If busy, just try again after a short wait
	if len(n.uncommitedKey) > 0 {
		n.Outgoing <- msg
		return
	}

	val, err := n.Store.Get(msg.Key)

	if err != nil {
		n.Outgoing <- packet.Message{
			Id:       msg.Id,
			Src:      n.id,
			Dest:     msg.Src,
			DemuxKey: packet.ClientReadResponse,
			Key:      msg.Key,
			Ok:       false,
		}
	} else {
		timestamp, val := decodeTimestampVal(val)

		n.Outgoing <- packet.Message{
			Id:        msg.Id,
			Src:       n.id,
			Dest:      msg.Src,
			DemuxKey:  packet.ClientReadResponse,
			Key:       msg.Key,
			Value:     val,
			Timestamp: timestamp,
			Ok:        true,
		}
	}
}

func (n *Dbnode) processLocalWrite(msg packet.Message) {
	// If busy, just try again after a short wait
	if len(n.uncommitedKey) > 0 {
		n.Outgoing <- msg
		return
	}

	oldVal, err := n.Store.Get(msg.Key)
	if err != nil {
		n.Outgoing <- packet.Message{
			Id:       msg.Id,
			Src:      n.id,
			Dest:     msg.Src,
			DemuxKey: packet.ClientWriteResponse,
			Key:      msg.Key,
			Value:    msg.Value,
			Ok:       false,
		}
		return
	}

	timestamp, oldVal := decodeTimestampVal(oldVal)
	timestamp++

	if timestamp != msg.Timestamp && msg.DemuxKey == packet.ClientStrongWriteRequest {
		n.Outgoing <- packet.Message{
			Id:        msg.Id,
			Src:       n.id,
			Dest:      msg.Src,
			DemuxKey:  packet.ClientWriteResponse,
			Key:       msg.Key,
			Value:     oldVal,
			Timestamp: timestamp,
			Ok:        false,
		}

		return
	}

	newVal := encodeTimestampVal(timestamp, msg.Value)

	txid := n.Store.Put(msg.Key, newVal)
	ok := n.Store.Commit(msg.Key, txid)

	n.Outgoing <- packet.Message{
		Id:        msg.Id,
		Src:       n.id,
		Dest:      msg.Src,
		DemuxKey:  packet.ClientWriteResponse,
		Key:       msg.Key,
		Value:     msg.Value,
		Timestamp: timestamp,
		Ok:        ok,
	}
}

func (n *Dbnode) handleLockRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && (n.currentMode == coordinatingRead || n.currentMode == assemblingQuorum) {
		if !msg.Ok {
			n.abortProcessing()
			return
		}

		// Make sure we don't count multiple responses from the same node
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
		}, true)
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

	n.unlockTxids[msg.Id] = true

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
	var timestamp uint64
	var ok bool

	if n.currentMode == processingRead && n.currentTxid == msg.Id || fastReads &&
		n.uncommitedKey == nil {
		var err error
		val, err = n.Store.Get(msg.Key)
		ok = err == nil
		if ok {
			timestamp, val = decodeTimestampVal(val)
		}
	}
	n.Outgoing <- packet.Message{
		Id:        msg.Id,
		Src:       n.id,
		Dest:      msg.Src,
		DemuxKey:  packet.NodeGetResponse,
		Key:       msg.Key,
		Value:     val,
		Timestamp: timestamp,
		Ok:        ok,
	}
}

func (n *Dbnode) handleGetRes(msg packet.Message) {
	n.requestRepeater.Ack(msg)

	if n.currentTxid == msg.Id && (n.currentMode == coordinatingRead ||
		n.currentMode == coordinatingWrite ||
		n.currentMode == coordinatingFastRead) {
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
		val := encodeTimestampVal(msg.Timestamp, msg.Value)

		n.uncommitedTxid = n.Store.Put(msg.Key, val)
		if n.uncommitedTxid > 0 {
			n.uncommitedKey = msg.Key
			ok = true
		}
	}

	n.Outgoing <- packet.Message{
		Id:        msg.Id,
		Src:       n.id,
		Dest:      msg.Src,
		DemuxKey:  packet.NodePutResponse,
		Key:       msg.Key,
		Value:     msg.Value,
		Timestamp: msg.Timestamp,
		Ok:        ok,
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
	var timestamp uint64

	if n.currentMode == processingWrite && n.currentTxid == msg.Id {
		val, _ = n.Store.Get(msg.Key)
		timestamp, _ = decodeTimestampVal(val)
	}

	n.Outgoing <- packet.Message{
		Id:        msg.Id,
		Src:       n.id,
		Dest:      msg.Src,
		DemuxKey:  packet.NodeGetResponse,
		Key:       msg.Key,
		Timestamp: timestamp,
		Ok:        true,
	}
}

func (n *Dbnode) handleBackgroundWriteReq(msg packet.Message) {
	currentVal, _ := n.Store.Get(msg.Key)
	currentTimestamp, currentVal := decodeTimestampVal(currentVal)

	if msg.Timestamp == currentTimestamp && !bytes.Equal(currentVal, msg.Value) {
		log.Fatal("Inconsistent values with same timestamp", currentVal, msg.Value)
	}
	if msg.Timestamp > currentTimestamp {
		value := encodeTimestampVal(msg.Timestamp, msg.Value)
		txid := n.Store.Put(msg.Key, value)
		n.Store.Commit(msg.Key, txid)
	}
	if msg.Timestamp >= currentTimestamp {
		n.Outgoing <- packet.Message{
			Id:        msg.Id,
			Src:       n.id,
			Dest:      msg.Src,
			DemuxKey:  packet.NodeBackgroundWriteResponse,
			Key:       msg.Key,
			Value:     msg.Value,
			Timestamp: msg.Timestamp,
			Ok:        true,
		}

		if n.logWrites {
			log.Println(n.id, "background write", msg.Key, msg.Timestamp)
		}
	} else {
		n.Outgoing <- packet.Message{
			Id:        msg.Id,
			Src:       n.id,
			Dest:      msg.Src,
			DemuxKey:  packet.NodeBackgroundWriteResponse,
			Key:       msg.Key,
			Value:     currentVal,
			Timestamp: currentTimestamp,
			Ok:        false,
		}
	}
}

func (n *Dbnode) handleBackgroundWriteRes(msg packet.Message) {
	n.backgroundWriteDaemon.response(msg)

	if !msg.Ok {
		currentVal, _ := n.Store.Get(msg.Key)
		currentTimestamp, currentVal := decodeTimestampVal(currentVal)

		if msg.Timestamp == currentTimestamp && !bytes.Equal(currentVal, msg.Value) {
			log.Fatal("Inconsistent values with same timestamp", currentVal, msg.Value)
		}
		if msg.Timestamp > currentTimestamp {
			value := encodeTimestampVal(msg.Timestamp, msg.Value)
			txid := n.Store.Put(msg.Key, value)
			n.Store.Commit(msg.Key, txid)
		}
	}
}

func (n *Dbnode) continueProcessing() {
	switch n.currentMode {
	case coordinatingFastRead:
		if n.quorumMembers == nil {
			n.assembleQuorum(n.readQuorumSize, packet.NodeGetRequest)

			return
		}

		localVal, err := n.Store.Get(n.clientRequest.Key)
		if err != nil {
			n.abortProcessing()
			return
		}

		timestamp, value := decodeTimestampVal(localVal)

		for _, node := range n.quorumMembers {
			if node.Timestamp > timestamp {
				timestamp = node.Timestamp
				value = node.Value
			}
		}

		n.Outgoing <- packet.Message{
			Id:        n.clientRequest.Id,
			Src:       n.id,
			Dest:      n.clientRequest.Src,
			DemuxKey:  packet.ClientReadResponse,
			Key:       n.clientRequest.Key,
			Value:     value,
			Timestamp: timestamp,
			Ok:        true,
		}

		n.currentMode = idle
		n.currentTxid = -1
		n.quorumMembers = nil
		n.numWaitingNodes = 0
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
				}, false)
			}

			n.quorumMembers[n.id] = packet.Message{
				DemuxKey: packet.NodeUnlockRequest,
			}

			n.numWaitingNodes = n.readQuorumSize - 1
		case packet.NodeUnlockRequest:
			// Read local value
			localVal, err := n.Store.Get(n.clientRequest.Key)
			if err != nil {
				n.abortProcessing()
				return
			}

			timestamp, value := decodeTimestampVal(localVal)

			// Find the most recent value

			for id, res := range n.quorumMembers {
				if id == n.id {
					continue
				}

				t, v := decodeTimestampVal(res.Value)
				if t > timestamp {
					timestamp = t
					value = v
				}

				// Unlock each node
				n.requestRepeater.Send(packet.Message{
					Id:       res.Id,
					Src:      n.id,
					Dest:     id,
					DemuxKey: packet.NodeUnlockRequest,
					Ok:       true,
				}, true)
			}

			// Return to client
			n.Outgoing <- packet.Message{
				Id:        n.clientRequest.Id,
				Src:       n.id,
				Dest:      n.clientRequest.Src,
				DemuxKey:  packet.ClientReadResponse,
				Key:       n.clientRequest.Key,
				Value:     value,
				Timestamp: timestamp,
				Ok:        true,
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
				}, true)
			}

			n.quorumMembers[n.id] = packet.Message{
				DemuxKey: packet.NodePutRequest,
			}
			n.numWaitingNodes = n.writeQuorumSize - 1
		case packet.NodePutRequest:
			var latestTimestamp uint64

			localVal, err := n.Store.Get(n.clientRequest.Key)
			if err != nil {
				n.abortProcessing()
				return
			}

			if len(localVal) > 0 {
				latestTimestamp, _ = decodeTimestampVal(localVal)
			}

			for id, msg := range n.quorumMembers {
				if id == n.id {
					continue
				}

				if msg.Timestamp > latestTimestamp {
					latestTimestamp = msg.Timestamp
				}
			}

			if n.clientRequest.DemuxKey == packet.ClientStrongWriteRequest && n.clientRequest.Timestamp != latestTimestamp+1 {
				n.clientRequest.Timestamp = latestTimestamp + 1
				n.abortProcessing()
				return
			}

			value := encodeTimestampVal(latestTimestamp+1, n.clientRequest.Value)

			n.uncommitedTxid = n.Store.Put(n.clientRequest.Key, value)
			n.uncommitedKey = n.clientRequest.Key

			for id := range n.quorumMembers {
				if id == n.id {
					continue
				}

				n.requestRepeater.Send(packet.Message{
					Id:        n.clientRequest.Id,
					Src:       n.id,
					Dest:      id,
					DemuxKey:  packet.NodePutRequest,
					Key:       n.clientRequest.Key,
					Value:     n.clientRequest.Value,
					Timestamp: latestTimestamp + 1,
					Ok:        true,
				}, true)
			}

			n.quorumMembers[n.id] = packet.Message{
				DemuxKey:  packet.NodeUnlockRequest,
				Timestamp: latestTimestamp + 1,
			}
			n.numWaitingNodes = n.writeQuorumSize - 1
		case packet.NodeUnlockRequest:
			ok := n.Store.Commit(n.uncommitedKey, n.uncommitedTxid)
			if !ok {
				n.abortProcessing()
				return
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
				}, true)
			}

			n.Outgoing <- packet.Message{
				Id:        n.clientRequest.Id,
				Src:       n.id,
				Dest:      n.clientRequest.Src,
				DemuxKey:  packet.ClientWriteResponse,
				Key:       n.clientRequest.Key,
				Value:     n.clientRequest.Value,
				Timestamp: n.quorumMembers[n.id].Timestamp,
				Ok:        true,
			}

			if n.logWrites {
				log.Println(n.id, "write commit", n.clientRequest.Key, n.quorumMembers[n.id].Timestamp)
			}

			if n.backgroundWriteDaemon != nil {
				n.backgroundWriteDaemon.propagateTransaction(
					n.clientRequest.Id,
					n.quorumMembers,
					n.clientRequest.Key,
					n.clientRequest.Value,
					n.quorumMembers[n.id].Timestamp)
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
	if n.uncommitedTxid > 0 {
		n.Store.Rollback(n.uncommitedTxid)
		n.uncommitedTxid = 0
		n.uncommitedKey = nil
	}

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
			}, true)
		}
	}

	switch n.currentMode {
	case assemblingQuorum, coordinatingRead, coordinatingWrite:
		var resType packet.Messagetype
		if n.currentMode == coordinatingRead {
			resType = packet.ClientReadResponse
		} else {
			resType = packet.ClientWriteResponse
		}

		n.Outgoing <- packet.Message{
			Id:        n.clientRequest.Id,
			Src:       n.id,
			Dest:      n.clientRequest.Src,
			DemuxKey:  resType,
			Key:       n.clientRequest.Key,
			Value:     n.clientRequest.Value,
			Timestamp: n.clientRequest.Timestamp,
			Ok:        false,
		}
	case processingRead, processingWrite:
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

	var key []byte
	var val []byte
	if requestType == packet.NodeGetRequest {
		key = n.clientRequest.Key
		val = n.clientRequest.Value
	}

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
			Key:      key,
			Value:    val,
			Ok:       true,
		}
		n.requestRepeater.Send(n.quorumMembers[node], false)
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

// QueryState is for debugging/monitoring purposes. It returns currentTxid.
func (n *Dbnode) QueryState() int {
	n.stateQueryReq <- true
	return <-n.stateQueryRes
}
