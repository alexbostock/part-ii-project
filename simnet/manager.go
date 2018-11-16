package simnet

import "log"

type link struct {
	incoming chan message
	outgoing chan message
}

func Simulate(numNodes uint) {
	links := make([]link, numNodes+1)

	var i uint
	for i = 0; i < numNodes; i++ {
		links[i] = link{
			make(chan message, 25),
			make(chan message, 25),
		}

		go startNode(int(i), links[i].incoming, links[i].outgoing)
		go startHelper(links[i].outgoing, links)
	}

	// Address numNodes is the "client" address, used by the manager
	links[numNodes] = link{
		make(chan message, 25),
		make(chan message, 25),
	}

	go sendTests(numNodes, links[numNodes].outgoing)
	go startHelper(links[numNodes].outgoing, links)

	recordClientResponses(numNodes, links[numNodes].incoming)
}

func startHelper(outgoing chan message, links []link) {
	for msg := range outgoing {
		if msg.dest < len(links) {
			// Deliver immediately for now. Add latency later.
			links[msg.dest].incoming <- msg
		} else {
			log.Printf("Misaddressed message from %d to %d", msg.src, msg.dest)
		}
	}
}

func sendTests(numNodes uint, outgoing chan message) {
	var i uint
	for i = 0; i < numNodes; i++ {
		msg := message{
			int(numNodes),
			int(i),
			clientReadRequest,
			make([]byte, 0),
			make([]byte, 0),
			true,
		}

		log.Printf("Request sent\t%+v", msg)
		outgoing <- msg
	}
}

func recordClientResponses(numNodes uint, incoming chan message) {
	// Need to wait for as many responses as client requests sent
	// For now, this equal to numNodes, but will change later
	var i uint
	for i = 0; i < numNodes; i++ {
		log.Printf("Response received\t%+v", <-incoming)
	}
}
