package net

import "sync"

type partitions struct {
	unavailableLinks map[int]map[int]bool
	// node -> node -> unavailable?
	// 1st node id <= 2nd node id

	lock sync.RWMutex
}

func newPartitions(n int) *partitions {
	p := &partitions{
		unavailableLinks: make(map[int]map[int]bool),
	}

	for i := 0; i <= n; i++ {
		p.unavailableLinks[i] = make(map[int]bool)
	}

	return p
}

func (p *partitions) linkAvailable(s, d int) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return !p.unavailableLinks[s][d]
}

func (p *partitions) createPartition(links map[int]map[int]bool) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for source, dests := range links {
		for dest := range dests {
			p.unavailableLinks[source][dest] = true
		}
	}
}

func (p *partitions) removePartition(links map[int]map[int]bool) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for source, dests := range links {
		for dest := range dests {
			delete(p.unavailableLinks[source], dest)
		}
	}
}
