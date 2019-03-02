package dbnode

import (
	"github.com/alexbostock/part-ii-project/packet"
)

type queue struct {
	values []*packet.Message
}

func (q *queue) enqueue(msg *packet.Message) {
	if !q.contains(msg) {
		q.values = append(q.values, msg)
	}
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

func (q *queue) contains(msg *packet.Message) bool {
	for _, m := range q.values {
		if m == msg {
			return true
		}
	}

	return false
}

func (q *queue) length() int {
	return len(q.values)
}
