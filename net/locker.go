package net

import (
	"strconv"
	"time"
)

// A ClientLocker is a primitive used to acquire a distributed lock using my
// system. It must be instantiated using NewClientLocker. It requires a Client
// and database nodes to function. Lock and Unlock must not be called
// concurrently. The key {1, 0} in the database must not be used except by
// ClientLockers. If availability of database nodes is poor, or if the lock is
// held by other clients, calls to Lock and Unlock may block for a long time.
// This implementation only provides a single lock. Change the key used to use
// multiple locks. Every key used for locking may not be used for any other
// purpose. Locks are advisory.
type ClientLocker struct {
	key    []byte
	id     int
	client *Client

	wantLock bool
	haveLock bool

	lockResChan   chan bool
	unlockResChan chan bool
}

// NewClientLocker instantiates a ClientLocker. Its argument are a client ID
// (which must be unique), and a Client (see client.go).
func NewClientLocker(id int, client *Client) *ClientLocker {
	cl := &ClientLocker{
		key: []byte{1, 0},

		id:     id,
		client: client,

		wantLock: false,
		haveLock: false,
	}

	go cl.daemon()

	return cl
}

func (cl *ClientLocker) daemon() {
	for {
		val, ts, ok := cl.client.Get(cl.key)
		if !ok {
			continue
		}

		nodeId, _ := strconv.Atoi(string(val))

		if ts%2 == 1 && nodeId == cl.id && !cl.wantLock {
			cl.haveLock = true

			res, _ := cl.client.StrongPut(cl.key, val, ts+1)
			if res == Success {
				cl.haveLock = false
				if cl.unlockResChan != nil {
					cl.unlockResChan <- true
					cl.unlockResChan = nil
				}
			}
		}

		if ts%2 == 0 && cl.wantLock {
			res, _ := cl.client.StrongPut(cl.key, []byte(strconv.Itoa(cl.id)), ts+1)
			if res == Success {
				cl.haveLock = true
				if cl.lockResChan != nil {
					cl.lockResChan <- true
					cl.lockResChan = nil
				}
			}
		}

		time.Sleep(time.Second / 5)
	}
}

// Lock is used to acquire the lock. Calls will block until the lock has been
// acquired. Lock must not be called concurrently with other Lock calls, or
// with Unlock calls. Lock must not be called when the lock is already held.
func (cl *ClientLocker) Lock() {
	c := make(chan bool)
	cl.lockResChan = c
	cl.wantLock = true
	<-c
}

// Lock is used to release the lock. Calls will block until the lock has been
// released. Unlock must not be called concurrently with other Unlock calls, or
// with Lock calls. Unlock must not be called when the lock is not already held.
func (cl *ClientLocker) Unlock() {
	c := make(chan bool)
	cl.unlockResChan = c
	cl.wantLock = false
	<-c
}
