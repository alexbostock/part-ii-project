package dbnode

import (
	"encoding/binary"
	"log"
)

func decodeTimestampVal(encoded []byte) (timestamp uint64, value []byte) {
	// First 11 bytes are Lamport timestamp (in base 64)

	if len(encoded) < 8 {
		log.Fatalf("Invalid value stored: value prefix must be a 64 bit Lamport timestamp.\n%v", encoded)
	}

	timestamp = binary.BigEndian.Uint64(encoded[:8])

	value = encoded[8:]

	return
}
