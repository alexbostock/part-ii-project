package elector

import "github.com/alexbostock/part-ii-project/net/packet"

// A Dummy is a do-nothing elector, equivalent to no election algorithm
type Dummy struct {
	id       int
	outgoing chan packet.Message
}

func newDummy(id int, outgoing chan packet.Message) *Dummy {
	return &Dummy{
		id:       id,
		outgoing: outgoing,
	}
}

func (d *Dummy) Leader() int {
	return d.id
}

func (d *Dummy) ForwardToLeader(msg packet.Message) {
	msg.Dest = d.id
	d.outgoing <- msg
}

func (d *Dummy) ProcessMsg(msg packet.Message) {
	// Do nothing
}
