package datastore

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// A persistentstore is a persistent data store which stores data in binary
// in a directory. It is essentially a persistent hash map.
type persistentstore struct {
	path string
	txid int
}

func validatePage(page [][]byte) {
	// A data file should match (key \0 value \0)*
	// After bytes.Split(data, 0), we should have an odd number of slices, with
	// the last one being empty.

	if len(page)%2 == 0 {
		log.Fatal("Corrupt datastore: every key must have a value")
	}

	if len(page[len(page)-1]) > 0 {
		log.Fatal("Corrupt datastore: each page must end with nil")
	}
}

// Get attempts to retreive the value associated with a key. The key must not
// contain any 0 (null) bytes. Get returns nil and an error on failure. It may
// return nil with a nil error, indicating that the requested value is not
// present in the store.
func (store *persistentstore) Get(key []byte) ([]byte, error) {
	sum := md5.Sum(key)
	hash := hex.EncodeToString(sum[:])
	path := filepath.Join(store.path, hash)

	// If there is no file, we do not have a value for the requested key
	// (this is not an error)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, e := ioutil.ReadFile(path)

	// Any other io error is an error
	if e != nil {
		return nil, errors.New("Failed to read from disk")
	}

	values := bytes.Split(data, []byte{0})

	validatePage(values)

	for i := 0; i < len(values)-1; i += 2 {
		if bytes.Equal(values[i], key) {
			return values[i+1], nil
		}
	}

	// EOF reached means key not present (which is not an error)
	return nil, nil
}

// Put attempts to store (but not commit) a key, value pair in the store. If
// successful, it returns a unique non-zero transaction ID. In case of error,
// it returns -1.
func (store *persistentstore) Put(key, val []byte) int {
	store.txid++
	if store.txid == 0 {
		store.txid++
	}

	sum := md5.Sum(key)
	hash := hex.EncodeToString(sum[:])

	data, e := ioutil.ReadFile(filepath.Join(store.path, hash))
	if os.IsNotExist(e) {
		return store.putNew(key, val)
	} else if e != nil {
		return 0
	}

	newPage, e := os.Create(filepath.Join(store.path, "tx"+strconv.Itoa(store.txid)))
	if e != nil {
		return 0
	}
	defer newPage.Close()

	oldPage := bytes.Split(data, []byte{0})

	validatePage(oldPage)

	written := false

	for i := 0; i < len(oldPage)-1; i++ {
		if bytes.Equal(oldPage[i], key) {
			// Write the new key-value pair to newPage
			record := append(key, 0)
			record = append(record, val...)
			record = append(record, 0)

			_, e = newPage.Write(record)
			if e != nil {
				return 0
			}

			i++

			written = true
		} else {
			// Copy existing key-value pair to newPage
			record := append(oldPage[i], 0)
			i++
			record = append(record, oldPage[i]...)
			record = append(record, 0)

			_, e = newPage.Write(record)
			if e != nil {
				return 0
			}
		}
	}

	// Case of hash collision, where file already exists, but does not
	// contain the required key.
	if !written {
		record := append(key, 0)
		record = append(record, val...)
		record = append(record, 0)

		_, e = newPage.Write(record)
		if e != nil {
			return 0
		}
	}

	return store.txid
}

// Used internally in the case Put is writing a new key, rather than overwriting
func (store *persistentstore) putNew(key []byte, val []byte) int {
	newPage, e := os.Create(filepath.Join(store.path, "tx"+strconv.Itoa(store.txid)))
	if e != nil {
		return 0
	}
	defer newPage.Close()

	record := append(key, 0)
	record = append(record, val...)
	record = append(record, 0)

	_, e = newPage.Write(record)

	if e == nil {
		return store.txid
	} else {
		return 0
	}
}

// Commit commits an uncommitted transaction. It requires a transaction ID from
// Put which has not yet been committed or rolled back. The given key must
// match the id (from the call to Put).
func (store *persistentstore) Commit(key []byte, id int) bool {
	oldPath := filepath.Join(store.path, "tx"+strconv.Itoa(id))

	sum := md5.Sum(key)
	newPath := filepath.Join(store.path, hex.EncodeToString(sum[:]))

	e := os.Rename(oldPath, newPath)

	return e == nil
}

// DeleteStore removes all data associated with this store from the file system
// (deleting the directory this store uses).
func (store *persistentstore) DeleteStore() {
	os.RemoveAll(store.path)
}

// Rollback deletes an uncommitted transaction. It requires a transaction ID
// returned by Put which has not yet been committed or rolled back.
func (store *persistentstore) Rollback(id int) {
	path := filepath.Join(store.path, "tx"+strconv.Itoa(id))
	os.Remove(path)
}
