package net

import (
	"bytes"
	"testing"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/packet"
)

func TestDatabase(t *testing.T) {
	numNodes := 5
	quorumSize := uint(numNodes/2 + 1)
	timeout := 500 * time.Millisecond

	nodes := make([]*dbnode.Dbnode, 6)
	failedNodes := failed{nodes: make(map[int]bool)}

	for i := 0; i < numNodes; i++ {
		nodes[i] = dbnode.New(numNodes, i, timeout, false, quorumSize, quorumSize)
		go startHelper(nodes[i].Outgoing, nodes, 0, 0, failedNodes)
	}

	nodes[numNodes] = &dbnode.Dbnode{
		Incoming: make(chan packet.Message, 100),
		Outgoing: make(chan packet.Message, 100),
	}
	go startHelper(nodes[numNodes].Outgoing, nodes, 0, 0, failedNodes)

	client := NewClient(nodes, timeout, 1)

	k := []byte{1}
	v := []byte{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	val, _, ok := client.Get(k)
	if len(val) > 0 {
		t.Error("Reading unwritten key should return no value.")
	}
	if !ok {
		t.Error("Value not present should not be an error.")
	}

	res, _ := client.Put(k, v)
	if res == Error {
		t.Error("Write transaction failed")
	}

	val, _, ok = client.Get(k)
	if !ok {
		t.Error("Read transaction failed")
	}
	if !bytes.Equal(v, val) {
		t.Error("Incorrect value read", val)
	}
}
