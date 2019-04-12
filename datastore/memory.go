package datastore

import (
	"bytes"
	"time"
)

// An in-memory datastore module

type pair struct {
	key   []byte
	value []byte
}

// The type memstore is an in-memory datastore, which implements Store. Since
// keys are variable length byte slices, it uses a trie data structure.
type memstore struct {
	value       []byte
	children    map[byte]*memstore
	uncommitted map[int]pair
	txid        int

	seekTime    time.Duration
	kbReadTime  time.Duration
	kbWriteTime time.Duration
}

// Get retrieves the requested value from the store. It has an err return value
// to match the Store interface, but err is always nil.
func (store *memstore) Get(key []byte) ([]byte, error) {
	t := time.NewTimer(time.Duration(len(key)/1000)*store.kbReadTime + store.seekTime)

	if len(key) == 0 {
		<-t.C
		t = time.NewTimer(time.Duration(len(store.value)/1000) * store.kbReadTime)
		<-t.C

		return store.value, nil
	}
	if store.children[key[0]] != nil {
		return store.children[key[0]].Get(key[1:])
	}

	<-t.C

	return nil, nil
}

// Put stores the requested value in the store and returns a unique, non-zero
// transaction ID. This operation is always successful.
func (store *memstore) Put(key, val []byte) int {
	t := time.NewTimer(time.Duration(len(val)/1000)*store.kbWriteTime + store.seekTime)

	store.txid++
	if store.txid == 0 {
		store.txid++
	}

	store.uncommitted[store.txid] = pair{key, val}

	<-t.C

	return store.txid
}

// Commit commits a transaction given a key and transaction ID. The ID must be
// the value returned by a previous call to Put.
func (store *memstore) Commit(key []byte, id int) bool {
	if !bytes.Equal(store.uncommitted[id].key, key) {
		return false
	}

	store.insert(key, store.uncommitted[id].value)
	delete(store.uncommitted, id)
	return true
}

// DeleteStore does nothing: an in-memory store can just be left for the
// garbage collector. This methods exists solely to satisfy the requirements of
// the Store interface.
func (store *memstore) DeleteStore() {
	// Do nothing
}

// Rollback removes an uncommitted transaction. This requires a transaction ID
// returned by Put which has not been Committed or Rollbacked.
func (store *memstore) Rollback(id int) {
	delete(store.uncommitted, id)
}

func (store *memstore) insert(key, value []byte) {
	if len(key) == 0 {
		store.value = value
	} else {
		if store.children[key[0]] == nil {
			store.children[key[0]] = &memstore{children: make(map[byte]*memstore)}
		}
		store.children[key[0]].insert(key[1:], value)
	}
}
