package simnet

import (
	"log"
)

func startNode(id int, incoming chan message, outgoing chan message) {
	for msg := range incoming {
		if msg.dest != id {
			log.Print("Misdelivered message for %d delivered to %d", msg.dest, id)
		} else {
			switch msg.demuxKey {
			case clientReadRequest:
				outgoing <- message{
					msg.id,
					id,
					msg.src,
					clientReadResponse,
					msg.key,
					make([]byte, 0),
					false,
				}
			case clientWriteRequest:
				outgoing <- message{
					msg.id,
					id,
					msg.src,
					clientWriteResponse,
					msg.key,
					msg.value,
					false,
				}
			default:
				log.Print("Unexpected message type received %+v", msg)
			}
		}
	}
}
