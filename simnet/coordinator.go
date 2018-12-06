package simnet

type coordinator interface {
	correctId(id int) bool

	nodeLocked(id int)
	nodeReturned(id int, key, value []byte)
	nodeUnlocked(id int)
	abort(bool)
}
