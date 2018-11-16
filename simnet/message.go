package simnet

type messagetype uint

const (
	_                             = iota
	clientReadRequest messagetype = iota
	clientWriteRequest
	clientReadResponse
	clientWriteResponse
)

type message struct {
	id       int
	src      int
	dest     int
	demuxKey messagetype
	key      []byte
	value    []byte
	ok       bool
}
