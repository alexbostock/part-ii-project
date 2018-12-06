package simnet

import (
	"log"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/alexbostock/part-ii-project/simnet/datastore"
)

type nodestate struct {
	id             int          // A unique node identifier
	incoming       chan message // Streams of network messages
	outgoing       chan message
	clientReqQueue chan message // Holds client request to avoid blocking incoming
	mutex          *sync.Mutex
	store          datastore.Store
}

func startNode(id int, incoming, outgoing chan message) {
	state := nodestate{
		id,
		incoming,
		outgoing,
		make(chan message, 25),
		&sync.Mutex{},
		datastore.CreateStore(filepath.Join("data", strconv.Itoa(id))),
	}

	go handleRequests(state)

	for msg := range incoming {
		if msg.dest != id {
			log.Print("Misdelivered message for %d delivered to %d", msg.dest, id)
		} else {
			switch msg.demuxKey {
			case clientReadRequest:
				state.clientReqQueue <- msg
			case clientWriteRequest:
				state.clientReqQueue <- msg
			default:
				log.Print("Unexpected message type received %+v", msg)
			}
		}
	}
}

func handleRequests(node nodestate) {
	for msg := range node.clientReqQueue {
		switch msg.demuxKey {
		case clientReadRequest:
			node.outgoing <- message{
				msg.id,
				node.id,
				msg.src,
				clientReadResponse,
				msg.key,
				make([]byte, 0),
				false,
			}
		case clientWriteRequest:
			node.outgoing <- message{
				msg.id,
				node.id,
				msg.src,
				clientWriteResponse,
				msg.key,
				msg.value,
				false,
			}
		}
	}
}
