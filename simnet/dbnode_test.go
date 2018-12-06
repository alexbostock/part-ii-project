package simnet

import (
	"os"
	"testing"
	"time"
)

type nodechans struct {
	incoming chan message
	outgoing chan message
}

var testNode nodechans

type messagepair struct {
	in  message
	out message
}

func TestMain(m *testing.M) {
	testNode = nodechans{
		make(chan message, 100),
		make(chan message, 100),
	}

	go startNode(5, 0, testNode.incoming, testNode.outgoing, 100*time.Millisecond)

	code := m.Run()
	os.Exit(code)
}

func TestDbnode(t *testing.T) {
	cases := []messagepair{
		messagepair{
			message{src: 1, demuxKey: nodeGetRequest, ok: true},
			message{dest: 1, demuxKey: nodeGetResponse, ok: false},
		},
		messagepair{
			message{src: 2, demuxKey: nodeUnlockRequest, ok: true},
			message{dest: 2, demuxKey: nodeUnlockAck, ok: true},
		},
	}

	for _, pair := range cases {
		testNode.incoming <- pair.in

		out := <-testNode.outgoing
		if !messagesEqual(out, pair.out) {
			t.Errorf("%+v returned %+v, want %+v", pair.in, out, pair.out)
		}
	}
}
