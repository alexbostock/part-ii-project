package datastore

type persistentstore string

func (store persistentstore) Get(key []byte) []byte {
	return nil
}

func (store persistentstore) Put(key []byte, val []byte) int {
	return 0
}

func (store persistentstore) Commit(id int) bool {
	return false
}

func (store persistentstore) DeleteStore() {
}
