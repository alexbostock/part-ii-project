package datastore

type datastore interface {
	Get(key []byte) []byte          // Returns a value, or nil to indicate failure
	Put(key []byte, val []byte) int // Returns a unique non-zero id if write was successful (requires a commit call to complete)
	Commit(id int) bool             // Returns true iff the transaction with id id was successfully committed
	DeleteStore()                   // Delete the store (including removing all data from disk)
}

func createStore(path string) datastore {
	if path == "" {
		// TODO: return an in-memory store in this case
		return nil
	} else {
		return persistentstore(path)
	}
}
