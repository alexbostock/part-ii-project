package datastore

import "bytes"

// An in-memory datastore module
// Keys can be variable length, so they have to be byte slices.
// Slices cannot be used as map keys, so we use a trie structure.

type pair struct {
	key   []byte
	value []byte
}

type memstore struct {
	value       []byte
	children    map[byte]*memstore
	uncommitted map[int]pair
	txid        int
}

func (store *memstore) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return store.value, nil
	}
	if store.children[key[0]] != nil {
		return store.children[key[0]].Get(key[1:])
	}
	return nil, nil
}

func (store *memstore) Put(key, val []byte) int {
	store.txid++
	if store.txid == 0 {
		store.txid++
	}

	store.uncommitted[store.txid] = pair{key, val}

	return store.txid
}

func (store *memstore) Commit(key []byte, id int) bool {
	if !bytes.Equal(store.uncommitted[id].key, key) {
		return false
	}

	store.insert(key, store.uncommitted[id].value)
	delete(store.uncommitted, id)
	return true
}

func (store *memstore) DeleteStore() {
	// Do nothing
}

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
