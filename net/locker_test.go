package net

import (
	"testing"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/net/packet"
)

func TestClientLocker(t *testing.T) {
	numNodes := 5
	quorumSize := uint(numNodes/2 + 1)
	timeout := 500 * time.Millisecond

	nodes := make([]*dbnode.Dbnode, 6)

	p := newPartitions(numNodes)

	for i := 0; i < numNodes; i++ {
		nodes[i] = dbnode.New(numNodes, i, timeout, false, quorumSize, quorumSize, false, true)
		go startHelper(nodes[i].Outgoing, nodes, 0, 0, nil, p)
	}

	nodes[numNodes] = &dbnode.Dbnode{
		Incoming: make(chan packet.Message, 100),
		Outgoing: make(chan packet.Message, 100),
	}
	go startHelper(nodes[numNodes].Outgoing, nodes, 0, 0, nil, p)

	client := NewClient(nodes, timeout, 1)

	clientLocker := NewClientLocker(0, client)

	clientLocker.Lock()
	clientLocker.Unlock()
}
