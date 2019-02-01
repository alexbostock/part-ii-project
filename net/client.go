// Package net provides implementation of all parts of te network simulation
// except for individual database nodes.
package net

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/packet"
)

// A Client is an interface to the remote database system. it should be
// instantiated using NewClient. All methods block until either a response is
// received from the remote coordinator, or a timeout lapses.
type Client struct {
	nodes       []*dbnode.Dbnode
	numNodes    int
	numAttempts int
	timeout     time.Duration

	idStream      chan int
	responseChans sync.Map
}

// NewClient creates a new Client. Its arguments are the list of database nodes
// and the time to wait before giving up on a transaction.
func NewClient(nodes []*dbnode.Dbnode, timeout time.Duration, numAttempts int) *Client {
	// The last 'node' is the client node

	c := &Client{
		nodes:       nodes,
		numNodes:    len(nodes) - 1,
		numAttempts: numAttempts,
		timeout:     3 * timeout,

		idStream: make(chan int),
	}

	go c.generateIds()
	go c.routeResponses()

	return c
}

func (c *Client) generateIds() {
	for i := 0; true; i++ {
		c.idStream <- i
	}
}

func (c *Client) routeResponses() {
	for msg := range c.nodes[c.numNodes].Incoming {
		resChan, ok := c.responseChans.Load(msg.Id)
		if !ok {
			// Missing response channel just means the request timed out.
			continue
		}
		responseChan, ok := resChan.(chan packet.Message)
		if !ok {
			log.Fatal("Wrong type in responseChans map in client")
		}

		responseChan <- msg
	}
}

// Get picks a random database node as coordinator, sends a ClientReadRequest
// to that node, and either returns the response or returns an error response
// when the request times out. The second return value ok is true iff the
// request was successful. If ok, the first return value is the value returned
// (which may be nil).
func (c *Client) Get(key []byte) ([]byte, bool) {
	for i := 0; i < c.numAttempts; i++ {
		id := <-c.idStream

		resChan := make(chan packet.Message)
		c.responseChans.Store(id, resChan)

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
		case msg := <-resChan:
			if msg.Ok {
				return msg.Value, msg.Ok
			}
		case <-timer.C:
			continue
		}

		c.responseChans.Delete(id)
	}

	return nil, false
}

// Put picks a random database node as coordinator, sends a ClientWriteRequest,
// and either returns whether the transaction was successful, or returns false
// if the requests times out.
func (c *Client) Put(key, val []byte) bool {
	for i := 0; i < c.numAttempts; i++ {
		id := <-c.idStream

		resChan := make(chan packet.Message)
		c.responseChans.Store(id, resChan)

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
		case msg := <-resChan:
			if msg.Ok {
				return true
			}
		case <-timer.C:
			continue
		}

		c.responseChans.Delete(id)
	}

	return false
}
