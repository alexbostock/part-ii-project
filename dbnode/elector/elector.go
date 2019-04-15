// Package elector implements a leadership election.
package elector

import "github.com/alexbostock/part-ii-project/net/packet"

// An Elector acts as a node in an election. it should be instantiated using New
type Elector interface {
	// Leader returns the current elected leader, or -1 if an election is in progress.
	Leader() int
	// ForwardToLeader forwards a message to the current leader. If called during
	// an election, it buffers messages and forwards them once a leader is elected.
	ForwardToLeader(packet.Message)
	// ProcessMsg is a receiver for packets. It should be sent all Election messages
	// received by a Dbnode.
	ProcessMsg(msg packet.Message)
}

// New creates a new Elector (currently using the bully algorithm).
func New(id, n int, outgoing chan packet.Message) Elector {
	return newRing(id, n, outgoing)
}
