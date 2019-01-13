package simnet

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/alexbostock/part-ii-project/simnet/packet"
)

type logger time.Time

func (l logger) log(msg string) {
	fmt.Printf("%v\t%v\n", time.Since(time.Time(l)).Nanoseconds()/1000, msg)
}

type Options struct {
	NumNodes                    *uint
	RandomSeed                  *int64
	TransactionRate             *float64
	MeanMsgLatency              *float64
	MsgLatencyVariance          *float64
	NumTransactions             *uint
	ProportionWriteTransactions *float64
}

type link struct {
	incoming chan packet.Message
	outgoing chan packet.Message
}

func Simulate(o Options) {
	numNodes := *o.NumNodes

	rand.Seed(*o.RandomSeed)

	links := make([]link, numNodes+1)

	var i uint
	for i = 0; i < numNodes; i++ {
		links[i] = link{
			make(chan packet.Message, 1000),
			make(chan packet.Message, 1000),
		}

		// Timeout value hardcoded (1s)
		go startNode(int(numNodes), int(i), links[i].incoming, links[i].outgoing, 500*time.Millisecond)
		go startHelper(links[i].outgoing, links, *o.MeanMsgLatency, math.Sqrt(*o.MsgLatencyVariance))
	}

	// Address numNodes is the "client" address, used by the manager
	links[numNodes] = link{
		make(chan packet.Message, 1000),
		make(chan packet.Message, 1000),
	}

	timer := logger(time.Now())

	go sendTests(numNodes, links[numNodes].outgoing, timer, *o.NumTransactions, *o.TransactionRate, *o.ProportionWriteTransactions)
	go startHelper(links[numNodes].outgoing, links, *o.MeanMsgLatency, math.Sqrt(*o.MsgLatencyVariance))

	recordClientResponses(numNodes, links[numNodes].incoming, timer, *o.NumTransactions)
}

func startHelper(outgoing chan packet.Message, links []link, mean float64, stddev float64) {
	for msg := range outgoing {
		if msg.Dest < len(links) {
			// Normally distributed delay for now
			// TODO: better simulation of tcp latency

			delay := rand.NormFloat64()*stddev + mean

			go sendAfterDelay(msg, links[msg.Dest].incoming, time.Duration(delay)*time.Millisecond)
		} else {
			log.Printf("Misaddressed message from %d to %d", msg.Src, msg.Dest)
		}
	}
}

func sendAfterDelay(msg packet.Message, link chan packet.Message, delay time.Duration) {
	time.Sleep(delay)

	link <- msg
}

func sendTests(numNodes uint, outgoing chan packet.Message, l logger, numTransactions uint, transactionRate float64, proportionWrites float64) {
	var i uint
	for i = 0; i < numTransactions; i++ {
		dest := int(rand.Float64() * float64(numNodes))

		var msgType packet.Messagetype
		if rand.Float64() < proportionWrites {
			msgType = packet.ClientWriteRequest
		} else {
			msgType = packet.ClientReadRequest
		}

		// TODO: Parameterise key and value sizes
		key := make([]byte, 1)
		rand.Read(key)
		removeZeroBytes(key)

		var val []byte
		if msgType == packet.ClientWriteRequest {
			val = make([]byte, 8)
			rand.Read(val)
			removeZeroBytes(val)
		}

		msg := packet.Message{
			Id:       int(i),
			Src:      int(numNodes),
			Dest:     dest,
			DemuxKey: msgType,
			Key:      key,
			Value:    val,
			Ok:       true,
		}

		l.log(fmt.Sprintf("Request\t%+v", msg))
		outgoing <- msg

		time.Sleep(time.Duration(1000*rand.ExpFloat64()/transactionRate) * time.Millisecond)
	}
}

func recordClientResponses(numNodes uint, incoming chan packet.Message, l logger, numTransactions uint) {
	// Need to wait for as many responses as client requests sent
	// For now, this equal to numNodes, but will change later
	var i uint
	for i = 0; i < numTransactions; i++ {
		l.log(fmt.Sprintf("Response\t%+v", <-incoming))
	}
}

func removeZeroBytes(b []byte) {
	for i := 0; i < len(b); i++ {
		for b[i] == 0 {
			rand.Read(b[i : i+1])
		}
	}
}
