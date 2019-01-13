package datastore

import (
	"os"
)

type Store interface {
	Get(key []byte) []byte          // Returns a value, or nil to indicate failure
	Put(key, val []byte) int        // Returns a unique non-zero id if write was successful (requires a commit call to complete)
	Commit(key []byte, id int) bool // Returns true iff the transaction with id id was successfully committed
	DeleteStore()                   // Delete the store (including removing all data from disk)
	Rollback(id int)                // Deletes all traces of an uncommitted transaction
}

func New(path string) Store {
	if path == "" {
		return &memstore{nil, make(map[byte]*memstore), make(map[int]pair), 0}
	} else {
		os.Mkdir(path, 0755)
		return &persistentstore{path, 0}
	}
}
