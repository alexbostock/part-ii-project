package net

import (
	"math/rand"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/packet"
)

type Client struct {
	nodes    []*dbnode.Dbnode
	numNodes int
	timeout  time.Duration
}

func NewClient(nodes []*dbnode.Dbnode, timeout time.Duration) Client {
	// The last 'node' is the client node

	return Client{
		nodes,
		len(nodes) - 1,
		2 * timeout,
	}
}

func (c Client) Get(id int, key []byte) ([]byte, bool) {
	timer := time.NewTimer(c.timeout)

	dest := int(rand.Float64() * float64(c.numNodes))

	c.nodes[dest].Outgoing <- packet.Message{
		Id:       id,
		Src:      c.numNodes,
		Dest:     dest,
		DemuxKey: packet.ClientReadRequest,
		Key:      key,
		Ok:       true,
	}

	select {
	case <-timer.C:
		return nil, false
	case msg := <-c.nodes[c.numNodes].Incoming:
		return msg.Value, msg.Ok
	}
}

func (c Client) Put(id int, key, val []byte) bool {
	timer := time.NewTimer(c.timeout)

	dest := int(rand.Float64() * float64(c.numNodes))

	c.nodes[dest].Outgoing <- packet.Message{
		Id:       id,
		Src:      c.numNodes,
		Dest:     dest,
		DemuxKey: packet.ClientWriteRequest,
		Key:      key,
		Value:    val,
		Ok:       true,
	}

	select {
	case <-timer.C:
		return false
	case msg := <-c.nodes[c.numNodes].Incoming:
		return msg.Ok
	}
}
