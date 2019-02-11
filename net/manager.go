package net

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/packet"
)

type logger struct {
	startTime time.Time
	lock      sync.Mutex
}

func (l *logger) log(msg string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	fmt.Printf("%v\t%v\n", time.Since(l.startTime).Nanoseconds()/1000, msg)
}

// Options represents the parameters with which to run the system. These map
// directly to the command line options in main.
type Options struct {
	NumNodes                    *uint
	RandomSeed                  *int64
	TransactionRate             *float64
	MeanMsgLatency              *float64
	MsgLatencyVariance          *float64
	NodeFailureChance           *float64
	NumTransactions             *uint
	ProportionWriteTransactions *float64
	PersistentStore             *bool
	ReadQuorumSize              *uint
	WriteQuorumSize             *uint
	NumAttempts                 *uint
}

// Simulate starts database nodes, sets up the simulated network, and sends
// random client requests as tests, based on the given parameters.
func Simulate(o Options) {
	numNodes := *o.NumNodes

	rqs := *o.ReadQuorumSize
	wqs := *o.WriteQuorumSize

	if wqs <= numNodes/2 {
		log.Fatal("Write quorum size must greater than half the number of nodes")
	}

	rand.Seed(*o.RandomSeed)

	nodes := make([]*dbnode.Dbnode, numNodes+1)
	failedNodes := failed{nodes: make(map[int]bool)}

	// TODO: Parameterise timeout length
	timeout := 500 * time.Millisecond

	var i uint
	for i = 0; i < numNodes; i++ {
		nodes[i] = dbnode.New(int(numNodes), int(i), timeout, *o.PersistentStore, rqs, wqs)
		go startHelper(nodes[i].Outgoing, nodes, *o.MeanMsgLatency, math.Sqrt(*o.MsgLatencyVariance), failedNodes)
	}

	// Address numNodes is the "client" address, used by the manager
	nodes[numNodes] = &dbnode.Dbnode{
		Incoming: make(chan packet.Message, 1000),
		Outgoing: make(chan packet.Message, 1000),
	}

	timer := &logger{startTime: time.Now()}

	go startHelper(nodes[numNodes].Outgoing, nodes, *o.MeanMsgLatency, math.Sqrt(*o.MsgLatencyVariance), failedNodes)

	sendTests(nodes, timeout, timer, *o.NumTransactions, *o.TransactionRate, *o.ProportionWriteTransactions, *o.NodeFailureChance, failedNodes, *o.NumAttempts)

	for _, node := range nodes {
		if node.Store != nil {
			node.Store.DeleteStore()
		}
	}
}

func startHelper(outgoing chan packet.Message, links []*dbnode.Dbnode, mean float64, stddev float64, failedNodes failed) {
	for msg := range outgoing {
		if failedNodes.disabled(msg.Src) || failedNodes.disabled(msg.Dest) {
			continue
		}

		if msg.Dest < len(links) {
			// Normally distributed delay for now
			// TODO: better simulation of tcp latency

			delay := rand.NormFloat64()*stddev + mean

			go sendAfterDelay(msg, links[msg.Dest].Incoming, time.Duration(delay)*time.Millisecond)
		} else {
			log.Printf("Misaddressed message from %d to %d", msg.Src, msg.Dest)
		}
	}
}

func sendAfterDelay(msg packet.Message, link chan packet.Message, delay time.Duration) {
	time.Sleep(delay)

	link <- msg
}

func sendTests(nodes []*dbnode.Dbnode, timeout time.Duration, l *logger, numTransactions uint, transactionRate, proportionWrites, nodeFailureChance float64, failedNodes failed, numAttempts uint) {
	client := NewClient(nodes, timeout, int(numAttempts))

	var i uint
	for i = 0; i < numTransactions; i++ {
		// TODO: Parameterise key and value sizes
		key := make([]byte, 1)
		rand.Read(key)
		removeZeroBytes(key)

		// Chance of making a random node fail
		if rand.Float64() < nodeFailureChance {
			id := int(rand.Float64() * float64(len(nodes)-1))

			failedNodes.fail(id)
			nodes[id].Incoming <- packet.Message{
				DemuxKey: packet.ControlFail,
			}
		}

		// Chance of making a random node recover from failure
		if rand.Float64() < nodeFailureChance {
			id := failedNodes.unfailRand()

			if id >= 0 {
				nodes[id].Incoming <- packet.Message{
					DemuxKey: packet.ControlRecover,
				}
			}
		}

		if rand.Float64() < proportionWrites {
			val := make([]byte, 8)
			rand.Read(val)
			removeZeroBytes(val)

			go writeRequest(client, l, key, val)
		} else {
			go readRequest(client, l, key)
		}

		time.Sleep(time.Duration(1000*rand.ExpFloat64()/transactionRate) * time.Millisecond)
	}

	// Wait for the last responses before halting)
	time.Sleep(20 * timeout)
}

func writeRequest(c *Client, l *logger, key, val []byte) {
	l.log(fmt.Sprint("WriteRequest ", key, val))
	res, timestamp := c.Put(key, val)
	l.log(fmt.Sprint("WriteResponse ", key, val, timestamp, res))
}

func readRequest(c *Client, l *logger, key []byte) {
	l.log(fmt.Sprint("ReadRequest ", key))
	val, timestamp, ok := c.Get(key)
	l.log(fmt.Sprint("ReadResponse ", key, val, timestamp, ok))
}

func removeZeroBytes(b []byte) {
	for i := 0; i < len(b); i++ {
		for b[i] == 0 {
			rand.Read(b[i : i+1])
		}
	}
}
