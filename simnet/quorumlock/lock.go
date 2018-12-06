package quorumlock

import (
	"log"
	"time"
)

type reqtype int

const (
	unlockReq reqtype = iota
	lockReq
	lookup
)

type request struct {
	txid            int
	requestType     reqtype
	responseChannel chan bool
	reqid           uint64
}

type Lock struct {
	requests       chan request
	queuedRequests chan request
	currentHolder  int
	timeoutLength  time.Duration

	reqid             uint64
	lastTimedOutReqid uint64
	timeoutSigChan    chan uint64
}

func New(timeout time.Duration) *Lock {
	l := &Lock{
		requests:       make(chan request, 100),
		queuedRequests: make(chan request, 100),
		currentHolder:  -1,
		timeoutLength:  timeout,
		reqid:          1,
		timeoutSigChan: make(chan uint64, 100),
	}

	go l.handleRequests()

	return l
}

func (l *Lock) handleRequests() {
	for {
		select {
		case req := <-l.requests:
			switch req.requestType {
			case unlockReq:
				if req.txid == l.currentHolder {
					l.currentHolder = -1
					l.giveLockToQueuedReq()
				}
				req.responseChannel <- true
			case lockReq:
				if l.currentHolder == -1 {
					l.currentHolder = req.txid
					req.responseChannel <- true
				} else {
					req.reqid = l.reqid
					l.reqid++

					l.queuedRequests <- req

					go l.startTimer(req)
				}
			case lookup:
				req.responseChannel <- l.currentHolder == req.txid
			}
		case timedOut := <-l.timeoutSigChan:
			l.lastTimedOutReqid = timedOut
		default:
			time.Sleep(l.timeoutLength / 100)
		}
	}
}

func (l *Lock) giveLockToQueuedReq() {
	if l.currentHolder != -1 {
		log.Fatal("Quorum lock has reached an inconsistent state!")
	}

	select {
	case req := <-l.queuedRequests:
		if req.reqid > l.lastTimedOutReqid {
			l.currentHolder = req.txid
			req.responseChannel <- true
		} else {
			l.giveLockToQueuedReq()
		}
	default:
		// Do not block if there is no request waiting
		return
	}
}

func (l *Lock) startTimer(req request) {
	time.Sleep(l.timeoutLength / 2)

	req.responseChannel <- false

	l.timeoutSigChan <- req.reqid
}

func (l *Lock) Lock(id int) bool {
	resChan := make(chan bool, 1)

	l.requests <- request{
		txid:            id,
		requestType:     lockReq,
		responseChannel: resChan,
	}

	return <-resChan
}

func (l *Lock) Unlock(id int) {
	resChan := make(chan bool, 1)

	l.requests <- request{
		txid:            id,
		requestType:     unlockReq,
		responseChannel: resChan,
	}
}

func (l *Lock) HeldBy(id int) bool {
	resChan := make(chan bool, 1)

	l.requests <- request{
		txid:            id,
		requestType:     lookup,
		responseChannel: resChan,
	}

	return <-resChan
}
