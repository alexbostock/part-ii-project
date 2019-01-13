package datastore

import (
	"os"
)

type Store interface {
	Get(key []byte) []byte          // Returns a value, or nil to indicate failure
	Put(key []byte, val []byte) int // Returns a unique non-zero id if write was successful (requires a commit call to complete)
	Commit(key []byte, id int) bool // Returns true iff the transaction with id id was successfully committed
	DeleteStore()                   // Delete the store (including removing all data from disk)
}

func New(path string) Store {
	if path == "" {
		// TODO: return an in-memory store in this case
		return nil
	} else {
		os.Mkdir(path, 0755)
		return &persistentstore{path, 0}
	}
}
