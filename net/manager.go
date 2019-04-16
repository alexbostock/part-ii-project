package net

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/net/packet"
)

type logger struct {
	startTime time.Time
	lock      sync.Mutex
}

func (l *logger) log(startTime time.Duration, msg string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	t1 := startTime.Nanoseconds() / 1000
	t2 := l.timestamp().Nanoseconds() / 1000

	fmt.Printf("%v %v %v\n", t1, t2, msg)
}

func (l *logger) timestamp() time.Duration {
	return time.Since(l.startTime)
}

// Options represents the parameters with which to run the system. These map
// directly to the command line options in main.
type Options struct {
	NumNodes                    *uint
	RandomSeed                  *int64
	TransactionRate             *float64
	MeanMsgLatency              *float64
	MsgLatencyVariance          *float64
	NodeFailureRate             *float64
	MeanFailTime                *float64
	FailTimeVariance            *float64
	NumTransactions             *uint
	ProportionWriteTransactions *float64
	PersistentStore             *bool
	ReadQuorumSize              *uint
	WriteQuorumSize             *uint
	NumAttempts                 *uint
	SloppyQuorum                *bool
	ConvergenceTest             *bool
	LogWrites                   *bool
}

// Simulate starts database nodes, sets up the simulated network, and sends
// random client requests as tests, based on the given parameters.
func Simulate(o Options) {
	log.SetFlags(log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	sloppyQuorum := *o.SloppyQuorum || *o.ConvergenceTest

	numNodes := *o.NumNodes

	rqs := *o.ReadQuorumSize
	wqs := *o.WriteQuorumSize

	if rqs > numNodes {
		log.Fatal("Read quorum size must not be greater than the number of nodes.")
	}
	if wqs > numNodes {
		log.Fatal("Write quorum size must not be greater than the number of nodes.")
	}
	if wqs <= numNodes/2 {
		log.Fatal("Write quorum size must greater than half the number of nodes.")
	}
	if !sloppyQuorum && rqs+wqs <= numNodes {
		log.Fatal("Strict quorum requires V_R + V_W > n.")
	}
	if *o.TransactionRate <= 0 {
		log.Fatal("Transaction rate must be greater than 0.")
	}

	rand.Seed(*o.RandomSeed)

	nodes := make([]*dbnode.Dbnode, numNodes+1)

	monitor := newMonitor(nodes)

	// TODO: Parameterise timeout length
	timeout := 500 * time.Millisecond

	var i uint
	for i = 0; i < numNodes; i++ {
		nodes[i] = dbnode.New(int(numNodes), int(i), timeout, *o.PersistentStore, rqs, wqs, sloppyQuorum, *o.LogWrites)
	}

	// Address numNodes is the "client" address, used by the manager
	nodes[numNodes] = &dbnode.Dbnode{
		Incoming: make(chan packet.Message, 500),
		Outgoing: make(chan packet.Message, 500),
	}

	timer := &logger{startTime: time.Now()}

	partitionTracker := newPartitions(int(numNodes))

	// Start the network only after all nodes have been created to avoid
	// deferencing nil pointers
	for i = 0; i < numNodes; i++ {
		go startHelper(nodes[i].Outgoing, nodes, *o.MeanMsgLatency, math.Sqrt(*o.MsgLatencyVariance), monitor, partitionTracker)
	}

	go startHelper(nodes[numNodes].Outgoing, nodes, *o.MeanMsgLatency, math.Sqrt(*o.MsgLatencyVariance), monitor, partitionTracker)

	if *o.NodeFailureRate > 0 {
		go triggerNodeFailures(nodes, *o.NodeFailureRate, *o.MeanFailTime, *o.FailTimeVariance, timer, partitionTracker)
	}

	if *o.ConvergenceTest {
		go sendTests(nodes, timeout, timer, *o.NumTransactions, *o.TransactionRate*3/4, *o.ProportionWriteTransactions, *o.NumAttempts, monitor)
		sendConvergenceTests(nodes, timeout, timer, *o.NumTransactions/1000, monitor)
	} else {
		sendTests(nodes, timeout, timer, *o.NumTransactions, *o.TransactionRate, *o.ProportionWriteTransactions, *o.NumAttempts, monitor)
	}

	for _, node := range nodes {
		if node.Store != nil {
			node.Store.DeleteStore()
		}
	}
}

func startHelper(outgoing chan packet.Message, links []*dbnode.Dbnode, mean float64, stddev float64, m *monitor, p *partitions) {
	for msg := range outgoing {
		if msg.Dest < len(links) {
			if msg.Src < msg.Dest && !p.linkAvailable(msg.Src, msg.Dest) {
				continue
			}
			if msg.Dest < msg.Src && !p.linkAvailable(msg.Dest, msg.Src) {
				continue
			}

			delay := rand.NormFloat64()*stddev + mean

			go sendAfterDelay(msg, links[msg.Dest].Incoming, time.Duration(delay)*time.Millisecond)
		} else {
			log.Printf("Misaddressed message from %d to %d", msg.Src, msg.Dest)
		}
	}
}

func sendAfterDelay(msg packet.Message, link chan packet.Message, delay time.Duration) {
	time.Sleep(delay)

	select {
	case link <- msg:
		// If the destination buffer is not full, send the message
	default:
		// Else, discard the message
	}
}

func sendTests(nodes []*dbnode.Dbnode, timeout time.Duration, l *logger, numTransactions uint, transactionRate, proportionWrites float64, numAttempts uint, m *monitor) {
	client := NewClient(nodes, 10*timeout, int(numAttempts))

	var i uint
	for i = 0; i < numTransactions; i++ {
		key := make([]byte, 1)
		rand.Read(key)
		removeZeroBytes(key)

		if rand.Float64() < proportionWrites {
			val := make([]byte, 8)
			rand.Read(val)

			go writeRequest(client, l, key, val)
		} else {
			go readRequest(client, l, key)
		}

		time.Sleep(time.Duration(1000*rand.ExpFloat64()/transactionRate) * time.Millisecond)
	}

	// Wait for the last responses before halting)
	time.Sleep(20 * timeout)
}

func sendConvergenceTests(nodes []*dbnode.Dbnode, timeout time.Duration, l *logger, numTests uint, m *monitor) {
	client := NewClient(nodes, 10*timeout, 1)

	var i uint
	for i = 0; i < numTests; i++ {
		// Normal tests never have keys with 0 bytes, so always use key [0] to
		// distinguish convergence test transactions from others
		key := []byte{0}

		oldVal := make([]byte, 8)
		rand.Read(oldVal)

		newVal := make([]byte, 8)
		rand.Read(newVal)

		writeRequest(client, l, key, oldVal)
		readRequest(client, l, key)
		go writeRequest(client, l, key, newVal)

		for j := 0; j < 249; j++ {
			go readRequest(client, l, key)
			time.Sleep(4 * time.Millisecond)
		}

		readRequest(client, l, key)
	}

	time.Sleep(20 * timeout)
}

func triggerNodeFailures(nodes []*dbnode.Dbnode, failRate, mean, variance float64, l *logger, p *partitions) {
	stddev := math.Sqrt(variance)

	for {
		time.Sleep(time.Duration(rand.ExpFloat64()/(failRate/100)) * time.Second)

		// 50/50 chance of a single node failure or a partition
		if rand.Float64() < 0.5 {
			// Single node failure

			id := int(rand.Float64() * float64(len(nodes)-1))

			nodes[id].Incoming <- packet.Message{
				DemuxKey: packet.ControlFail,
			}

			delay := rand.NormFloat64()*stddev + mean

			go func(node *dbnode.Dbnode, delay time.Duration) {
				time.Sleep(delay)

				node.Incoming <- packet.Message{
					DemuxKey: packet.ControlRecover,
				}
			}(nodes[id], time.Duration(delay)*time.Second)
		} else {
			// Partition

			n := len(nodes)

			links := make(map[int]map[int]bool)

			for i := 0; i < n; i++ {
				s := rand.Intn(n)
				d := rand.Intn(n)

				if s < d {
					l := links[s]
					if l == nil {
						l = make(map[int]bool)
					}
					l[d] = true
					links[s] = l
				} else {
					l := links[d]
					if l == nil {
						l = make(map[int]bool)
					}
					l[s] = true
					links[d] = l
				}
			}

			fmt.Println("Partition created")

			p.createPartition(links) // map[int]map[int]bool

			delay := rand.NormFloat64()*stddev + mean

			go func(links map[int]map[int]bool, delay time.Duration) {
				time.Sleep(delay)

				fmt.Println("Partition recovered")

				p.removePartition(links)
			}(links, time.Duration(delay)*time.Second)
		}
	}
}

func writeRequest(c *Client, l *logger, key, val []byte) {
	startTime := l.timestamp()
	res, timestamp := c.Put(key, val)
	l.log(startTime, fmt.Sprint("write ", key, val, timestamp, res))
}

func readRequest(c *Client, l *logger, key []byte) {
	startTime := l.timestamp()
	val, timestamp, ok := c.Get(key)
	l.log(startTime, fmt.Sprint("read ", key, val, timestamp, ok))
}

func removeZeroBytes(b []byte) {
	for i := 0; i < len(b); i++ {
		for b[i] == 0 {
			rand.Read(b[i : i+1])
		}
	}
}
