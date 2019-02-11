package dbnode

import (
	"encoding/binary"
	"log"
)

func decodeTimestampVal(encoded []byte) (timestamp uint64, value []byte) {
	if len(encoded) == 0 {
		return 0, nil
	}

	// First 8 bytes are a Lamport timestamp.
	if len(encoded) < 8 {
		log.Fatalf("Invalid value stored: value prefix must be a 64 bit Lamport timestamp.\n%v", encoded)
	}

	timestamp = binary.BigEndian.Uint64(encoded[:8])

	value = encoded[8:]

	return
}

func encodeTimestampVal(timestamp uint64, value []byte) []byte {
	encoded := make([]byte, len(value)+8)
	binary.BigEndian.PutUint64(encoded[:8], timestamp)
	copy(encoded[8:], value)

	return encoded
}
