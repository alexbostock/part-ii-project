package simnet

import (
	"github.com/alexbostock/part-ii-project/simnet/packet"
)

type queue struct {
	values []*packet.Message
}

func (q *queue) enqueue(msg *packet.Message) {
	q.values = append(q.values, msg)
}

func (q *queue) dequeue() *packet.Message {
	msg := q.values[0]
	q.values = q.values[1:]

	return msg
}

func (q *queue) remove(msg *packet.Message) bool {
	for i, m := range q.values {
		if m == msg {
			q.values = append(q.values[:i], q.values[i+1:]...)
			return true
		}
	}

	return false
}

func (q *queue) empty() bool {
	return len(q.values) == 0
}