package net

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alexbostock/part-ii-project/dbnode"
	"github.com/alexbostock/part-ii-project/net/packet"
)

type monitor struct {
	nodes      []*dbnode.Dbnode
	msgTypes   map[int]map[packet.Messagetype]int // time -> Messagetype -> frequency
	nodeStates map[int]map[int]int                // time -> txid -> frequency

	msgStream chan packet.Messagetype

	ticker *time.Ticker
}

func newMonitor(nodes []*dbnode.Dbnode) *monitor {
	m := &monitor{
		nodes:      nodes,
		nodeStates: make(map[int]map[int]int),
		msgTypes:   make(map[int]map[packet.Messagetype]int),

		msgStream: make(chan packet.Messagetype),

		ticker: time.NewTicker(time.Second),
	}

	go m.monitorMsgTypes()
	go m.monitorNodeStates()

	return m
}

func (m *monitor) logMsg(msg packet.Message) {
	m.msgStream <- msg.DemuxKey
}

func (m *monitor) monitorMsgTypes() {
	counter := 0
	ticker := time.NewTicker(time.Second)

	m.msgTypes[counter] = make(map[packet.Messagetype]int)

	for {
		select {
		case <-ticker.C:
			counter++
			m.msgTypes[counter] = make(map[packet.Messagetype]int)
		case t := <-m.msgStream:
			count := m.msgTypes[counter][t]
			m.msgTypes[counter][t] = count + 1
		}
	}
}

func (m *monitor) monitorNodeStates() {
	counter := 0

	for {
		<-m.ticker.C

		m.nodeStates[counter] = make(map[int]int)

		for i, node := range m.nodes {
			if i == len(m.nodes)-1 {
				continue
			}

			txid := node.QueryState()

			count := m.nodeStates[counter][txid]
			m.nodeStates[counter][txid] = count + 1
		}

		counter++
	}
}

func (m *monitor) stop() {
	m.ticker.Stop()

	a, _ := json.Marshal(m.msgTypes)
	b, _ := json.Marshal(m.nodeStates)

	fmt.Println(string(a))
	fmt.Println(string(b))
}
