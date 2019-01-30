// Package net provides implementation of all parts of te network simulation
// except for individual database nodes.
package net

import (
	"math/rand"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/packet"
)

// A Client is an interface to the remote database system. it should be
// instantiated using NewClient. All methods block until either a response is
// received from the remote coordinator, or a timeout lapses.
type Client struct {
	nodes    []*dbnode.Dbnode
	numNodes int
	timeout  time.Duration
}

// NewClient creates a new Client. Its arguments are the list of database nodes
// and the time to wait before giving up on a transaction.
func NewClient(nodes []*dbnode.Dbnode, timeout time.Duration) Client {
	// The last 'node' is the client node

	return Client{
		nodes,
		len(nodes) - 1,
		5 * timeout,
	}
}

// Get picks a random database node as coordinator, sends a ClientReadRequest
// to that node, and either returns the response or returns an error response
// when the request times out. The second return value ok is true iff the
// request was successful. If ok, the first return value is the value returned
// (which may be nil).
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
	case msg := <-c.nodes[c.numNodes].Incoming:
		return msg.Value, msg.Ok
	case <-timer.C:
		return nil, false
	}
}

// Put picks a random database node as coordinator, sends a ClientWriteRequest,
// and either returns whether the transaction was successful, or returns false
// if the requests times out.
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
	case msg := <-c.nodes[c.numNodes].Incoming:
		return msg.Ok
	case <-timer.C:
		return false
	}
}
