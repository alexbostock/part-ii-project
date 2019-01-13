package dbnode

import (
	"encoding/base64"
	"encoding/binary"
	"log"
)

func decodeTimestampVal(encoded []byte) (timestamp uint64, value []byte) {
	// First 11 bytes are Lamport timestamp (in base 64)

	if len(encoded) < 11 {
		log.Fatalf("Invalid value stored: value prefix must be a 64 bit Lamport timestamp.\n%v", encoded)
	}

	timestampBytes := make([]byte, 8)
	base64.StdEncoding.Decode(timestampBytes, encoded[:11])
	timestamp = binary.BigEndian.Uint64(timestampBytes)

	value = encoded[11:]

	return
}
