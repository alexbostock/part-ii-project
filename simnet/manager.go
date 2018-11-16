package simnet

import (
	"fmt"
	"log"
	"time"
)

type logger time.Time

func (l logger) log(msg string) {
	fmt.Println(time.Since(time.Time(l)).Nanoseconds()/1000, "\t", msg)
}

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

	timer := logger(time.Now())

	go sendTests(numNodes, links[numNodes].outgoing, timer)
	go startHelper(links[numNodes].outgoing, links)

	recordClientResponses(numNodes, links[numNodes].incoming, timer)
}

func startHelper(outgoing chan message, links []link) {
	for msg := range outgoing {
		if msg.dest < len(links) {
			// 10ms latency for now, to be changed later
			go sendAfterDelay(msg, links[msg.dest].incoming, 10*time.Millisecond)
		} else {
			log.Printf("Misaddressed message from %d to %d", msg.src, msg.dest)
		}
	}
}

func sendAfterDelay(msg message, link chan message, delay time.Duration) {
	time.Sleep(delay)

	link <- msg
}

func sendTests(numNodes uint, outgoing chan message, l logger) {
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

		l.log(fmt.Sprintf("Request\t%+v", msg))
		outgoing <- msg

		// Send transactions at 10ms intervals (to be changed)
		time.Sleep(10 * time.Millisecond)
	}
}

func recordClientResponses(numNodes uint, incoming chan message, l logger) {
	// Need to wait for as many responses as client requests sent
	// For now, this equal to numNodes, but will change later
	var i uint
	for i = 0; i < numNodes; i++ {
		l.log(fmt.Sprintf("Response\t%+v", <-incoming))
	}
}
