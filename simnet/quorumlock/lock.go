package quorumlock

import (
	"log"
	"sync"
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
	timeout         bool
}

type Lock struct {
	requests       chan request
	queuedRequests chan request
	currentHolder  int
	timeoutLength  time.Duration

	reqid             uint64
	lastTimedOutReqid uint64
	lastTimedOutLock  *sync.Mutex
}

func New(timeout time.Duration) *Lock {
	l := &Lock{
		requests:         make(chan request, 100),
		queuedRequests:   make(chan request, 100),
		currentHolder:    -1,
		timeoutLength:    timeout,
		reqid:            1,
		lastTimedOutLock: &sync.Mutex{},
	}

	go l.handleRequests()

	return l
}

func (l *Lock) handleRequests() {
	for req := range l.requests {
		switch req.requestType {
		case unlockReq:
			l.lastTimedOutLock.Lock()
			if req.txid == l.currentHolder {
				l.currentHolder = -1
				l.giveLockToQueuedReq()
				req.responseChannel <- true
			} else {
				req.responseChannel <- false
			}
			l.lastTimedOutLock.Unlock()
		case lockReq:
			l.lastTimedOutLock.Lock()
			if l.currentHolder == -1 {
				l.currentHolder = req.txid
				req.responseChannel <- true
			} else {
				if req.timeout {
					req.reqid = l.reqid
					l.reqid++
					go l.startTimer(req)
				} else {
					req.reqid = 0
				}

				l.queuedRequests <- req
			}
			l.lastTimedOutLock.Unlock()
		case lookup:
			req.responseChannel <- l.currentHolder == req.txid
		}
	}
}

func (l *Lock) giveLockToQueuedReq() {
	if l.currentHolder != -1 {
		log.Fatal("Quorum lock has reached an inconsistent state!")
	}

	select {
	case req := <-l.queuedRequests:
		if req.reqid == 0 || req.reqid > l.lastTimedOutReqid {
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

	l.lastTimedOutLock.Lock()
	defer l.lastTimedOutLock.Unlock()

	if req.reqid > l.lastTimedOutReqid {
		l.lastTimedOutReqid = req.reqid
	}
	req.responseChannel <- false
}

func (l *Lock) Lock(id int, timesOut bool) bool {
	resChan := make(chan bool, 1)

	l.requests <- request{
		txid:            id,
		requestType:     lockReq,
		responseChannel: resChan,
		timeout:         timesOut,
	}

	return <-resChan
}

func (l *Lock) Unlock(id int) bool {
	resChan := make(chan bool, 1)

	l.requests <- request{
		txid:            id,
		requestType:     unlockReq,
		responseChannel: resChan,
	}

	return <-resChan
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
