package datastore

import (
	"bytes"
	"testing"
)

type testpair struct {
	key []byte
	val []byte
}

func TestPersistentStore(t *testing.T) {
	store := New("teststore")
	testStore(store, t)

	store = New("")
	testStore(store, t)
}

func testStore(store Store, t *testing.T) {
	defer store.DeleteStore()

	k := []byte{1, 2, 3}
	v := []byte{4, 5, 6, 7, 8, 9}
	v2 := []byte{4, 5, 6, 7, 8, 127}

	if store.Get(k) != nil {
		t.Error("Reading key not yet written should return nil.")
	}

	if store.Commit(k, 5) {
		t.Error("Committing non-existent transaction should return false.")
	}

	id := store.Put(k, v)

	if id == 0 {
		t.Error("Put should return a non-zero transaction id.")
	}

	if store.Get(k) != nil {
		t.Error("Not yet committed transaction should not be visible to Get.")
	}

	if !store.Commit(k, id) {
		t.Error("Commit with a valid id should commit and return true.")
	}

	if store.Get(k) == nil {
		t.Error("Value should be visible to Get after Put and Commit.")
	}

	id = store.Put(k, v2)

	if id == 0 {
		t.Error("Put should return a non-zero transaction id (even when overwriting a value.")
	}

	if !bytes.Equal(store.Get(k), v) {
		t.Error("Overwritten but not committed key should return the old value.")
	}

	if !store.Commit(k, id) {
		t.Error("Overwrite transaction commit failed.")
	}

	if !bytes.Equal(store.Get(k), v2) {
		t.Error("Overwritten value not returned by Get.")
	}

	// cases must not contain duplicate keys
	cases := []testpair{
		{[]byte{100, 61, 23, 44}, []byte{99, 30, 102, 121, 31, 104}},
		{[]byte{11, 57, 68, 84}, []byte{16, 12, 97, 124, 112, 21}},
		{[]byte{18, 119, 9, 116}, []byte{105, 78, 2, 72, 87, 6}},
		{[]byte{89, 108, 58, 41}, []byte{70, 83, 74, 35, 101, 50}},
		{[]byte{63, 114, 4, 86}, []byte{40, 47, 109, 46, 49, 82}},
		{[]byte{69, 76, 91, 92}, []byte{39, 77, 127, 3, 38, 111}},
		{[]byte{8, 5, 85, 96}, []byte{66, 60, 80, 106, 36, 55}},
		{[]byte{120, 81, 34, 15}, []byte{33, 1, 79, 26, 20, 110}},
		{[]byte{125, 45, 90, 43}, []byte{122, 54, 94, 27, 10, 107}},
		{[]byte{56, 29, 71, 103}, []byte{52, 73, 93, 51, 118, 24}},
		{[]byte{113, 117, 75, 32}, []byte{62, 14, 48, 7, 95, 25}},
		{[]byte{65, 67, 123, 59}, []byte{115, 53, 19, 13, 64, 17}},
		{[]byte{98, 88, 22, 37}, []byte{28, 126, 42, 104, 86, 10}},
		{[]byte{116, 24, 50, 20}, []byte{96, 47, 123, 108, 93, 21}},
		{[]byte{100, 52, 80, 127}, []byte{83, 82, 44, 67, 74, 115}},
		{[]byte{119, 117, 62, 6}, []byte{22, 16, 87, 8, 120, 65}},
		{[]byte{125, 34, 37, 45}, []byte{106, 92, 94, 99, 79, 71}},
		{[]byte{122, 36, 54, 49}, []byte{69, 72, 14, 11, 30, 112}},
		{[]byte{58, 38, 35, 33}, []byte{97, 3, 103, 42, 1, 107}},
		{[]byte{25, 121, 114, 63}, []byte{98, 111, 91, 68, 113, 64}},
		{[]byte{28, 56, 41, 57}, []byte{85, 48, 88, 101, 9, 81}},
		{[]byte{46, 51, 5, 66}, []byte{77, 73, 70, 23, 60, 89}},
		{[]byte{105, 15, 102, 53}, []byte{2, 26, 39, 17, 43, 32}},
		{[]byte{78, 27, 84, 55}, []byte{7, 4, 61, 110, 76, 29}},
	}

	for _, test := range cases {
		id = store.Put(test.key, test.val)
		if id == 0 {
			t.Error("Put should return a non-zero transaction id.")
		}
		if !store.Commit(test.key, id) {
			t.Error("Commit for a valid transaction should return true.")
		}
	}

	for _, test := range cases {
		if !bytes.Equal(store.Get(test.key), test.val) {
			t.Error("Incorrect value returned.")
		}
	}
}
