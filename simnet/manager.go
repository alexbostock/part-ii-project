package simnet

import "log"

type link struct {
	incoming chan message
	outgoing chan message
}

func Simulate(numNodes uint) {
	links := make([]link, numNodes)

	var i uint
	for i = 0; i < numNodes; i++ {
		links[i] = link{
			make(chan message, 25),
			make(chan message, 25),
		}

		go startNode(links[i].incoming, links[i].outgoing)

		go startHelper(links[i].outgoing, links)

		// TODO: Execute some tests on the main routine
	}
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
