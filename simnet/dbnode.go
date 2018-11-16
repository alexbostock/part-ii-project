package simnet

import "log"

func startNode(id int, incoming chan message, outgoing chan message) {
	for msg := range incoming {
		// Reply to every request with an empty response, for now
		if msg.dest != id {
			log.Print("Misdelivered message for %d delivered to %d", msg.dest, id)
		} else if msg.demuxKey == clientRequest {
			outgoing <- message{
				id,
				msg.src,
				clientResponse,
				msg.payload,
			}
		} else {
			log.Print("Unexpected message type received %d", msg.demuxKey)
		}
	}
}
