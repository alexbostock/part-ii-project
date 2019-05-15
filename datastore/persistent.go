package datastore

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io/ioutil"
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

// File format is (key_length key val_length val)*

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

	for len(data) > 0 {
		keyLen := binary.BigEndian.Uint32(data[:4])
		found := bytes.Equal(data[4:keyLen+4], key)

		data = data[keyLen+4:]

		valLen := binary.BigEndian.Uint32(data[:4])
		if found {
			return data[4 : valLen+4], nil
		}

		data = data[valLen+4:]
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

	oldPage, e := ioutil.ReadFile(filepath.Join(store.path, hash))
	if os.IsNotExist(e) {
		return store.putNew(key, val)
	} else if e != nil {
		return 0
	}

	newPage, e := os.Create(filepath.Join(store.path, "tx"+strconv.Itoa(store.txid)))
	if e != nil {
		return 0
	}
	defer newPage.Sync()
	defer newPage.Close()

	written := false

	for len(oldPage) > 0 {
		keyLen := binary.BigEndian.Uint32(oldPage[:4])
		found := bytes.Equal(oldPage[4:keyLen+4], key)

		_, e = newPage.Write(oldPage[:keyLen+4])
		if e != nil {
			return 0
		}

		oldPage = oldPage[keyLen+4:]

		valLen := binary.BigEndian.Uint32(oldPage[:4])
		if found {
			// Write new valLen and val
			record := make([]byte, len(val)+4)
			binary.BigEndian.PutUint32(record[:4], uint32(len(val)))
			copy(record[4:], val)

			_, e = newPage.Write(record)
			if e != nil {
				return 0
			}

			written = true
		} else {
			// Write current valLen and val
			_, e = newPage.Write(oldPage[:valLen+4])
		}

		oldPage = oldPage[valLen+4:]
	}

	// Case of hash collision, where file already exists, but does not
	// contain the required key.
	if !written {
		record := make([]byte, len(key)+len(val)+8)

		binary.BigEndian.PutUint32(record[:4], uint32(len(key)))
		copy(record[4:len(key)+4], key)

		binary.BigEndian.PutUint32(record[len(key)+4:len(key)+8], uint32(len(val)))
		copy(record[len(key)+8:], val)

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
	defer newPage.Sync()
	defer newPage.Close()

	record := make([]byte, len(key)+len(val)+8)

	binary.BigEndian.PutUint32(record[:4], uint32(len(key)))
	copy(record[4:len(key)+4], key)

	binary.BigEndian.PutUint32(record[len(key)+4:len(key)+8], uint32(len(val)))
	copy(record[len(key)+8:], val)

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
