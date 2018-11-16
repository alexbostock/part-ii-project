package simnet

type messagetype uint

const (
	_                         = iota
	clientRequest messagetype = iota
	clientResponse
)

type message struct {
	src      int
	dest     int
	demuxKey messagetype
	payload  int
}
