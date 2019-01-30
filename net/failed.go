package net

import "sync"

type failed struct {
	nodes map[int]bool
	lock  sync.Mutex
}

func (f *failed) fail(id int) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.nodes[id] = true
}

func (f *failed) unfail(id int) {
	f.lock.Lock()
	defer f.lock.Unlock()

	delete(f.nodes, id)
}

func (f *failed) unfailRand() int {
	f.lock.Lock()
	defer f.lock.Unlock()

	for node := range f.nodes {
		delete(f.nodes, node)
		return node
	}

	return -1
}

func (f *failed) unfailAll() {
	f.lock.Lock()
	defer f.lock.Unlock()

	for node := range f.nodes {
		delete(f.nodes, node)
	}
}

func (f *failed) disabled(id int) bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.nodes[id]
}
