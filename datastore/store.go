// Package datastore provides local key-value stores which support CRUD
// operations. There are several different implementations. All writes must be
// committed before they can be read.
package datastore

import (
	"os"
	"time"
)

// A Store is a local key-value store. There are currently two different
// implementations, which store data on disk or in memory. Keys may not contain
// any null bytes. Value lengths must be respresentable by a uint32.
type Store interface {
	Get(key []byte) ([]byte, error) // Returns a value, or nil to indicate no value
	Put(key, val []byte) int        // Returns a unique non-zero id if write was successful (requires a commit call to complete)
	Commit(key []byte, id int) bool // Returns true iff the transaction with id id was successfully committed
	DeleteStore()                   // Delete the store (including removing all data from disk)
	Rollback(id int)                // Deletes all traces of an uncommitted transaction
}

// New creates a new Store. Given the empty string, it creates an in-memory
// store. Given any other string, it attempts to use that string as the path
// to a data directory.
func New(path string) Store {
	if path == "" {
		return &memstore{
			nil,
			make(map[byte]*memstore),
			make(map[int]pair),
			0,

			310 * time.Microsecond,
			time.Second / 150000,
			time.Second / 285000,
		}
	} else {
		os.Mkdir(path, 0755)
		return &persistentstore{path, 0}
	}
}
