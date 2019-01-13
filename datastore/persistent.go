package datastore

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

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

func (store persistentstore) Get(key []byte) []byte {
	sum := md5.Sum(key)
	hash := hex.EncodeToString(sum[:])

	data, e := ioutil.ReadFile(filepath.Join(store.path, hash))
	if e != nil {
		return nil
	}

	values := bytes.Split(data, []byte{0})

	validatePage(values)

	for i := 0; i < len(values)-1; i += 2 {
		if bytes.Equal(values[i], key) {
			return values[i+1]
		}
	}

	// Error indicates end of file: key not found
	return nil
}

func (store persistentstore) Put(key []byte, val []byte) int {
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
func (store persistentstore) putNew(key []byte, val []byte) int {
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

func (store persistentstore) Commit(key []byte, id int) bool {
	oldPath := filepath.Join(store.path, "tx"+strconv.Itoa(id))

	sum := md5.Sum(key)
	newPath := filepath.Join(store.path, hex.EncodeToString(sum[:]))

	e := os.Rename(oldPath, newPath)

	return e == nil
}

func (store persistentstore) DeleteStore() {
	os.RemoveAll(store.path)
}
