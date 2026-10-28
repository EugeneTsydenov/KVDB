package protocol

import (
	"encoding/binary"
	"math"
)

// Appends a single byte.
func appendU8(b []byte, v uint8) []byte {
	return append(b, v)
}

// Appends a 2-byte unsigned integer in big-endian order.
func appendU16(b []byte, v uint16) []byte {
	return binary.BigEndian.AppendUint16(b, v)
}

// Appends an 8-byte unsigned integer in big-endian order.
func appendU64(b []byte, v uint64) []byte {
	return binary.BigEndian.AppendUint64(b, v)
}

// Appends an 8-byte signed integer in big-endian order.
func appendI64(b []byte, v int64) []byte {
	return binary.BigEndian.AppendUint64(b, uint64(v))
}

// Appends the data length as a u16, followed by the data.
// Returns ErrTooLarge if the data exceeds the maximum u16 length.
func appendBytes(b []byte, data []byte) ([]byte, error) {
	if len(data) > math.MaxUint16 {
		return b, ErrTooLarge
	}

	b = appendU16(b, uint16(len(data)))

	return append(b, data...), nil
}

// Appends a string in the same format as appendBytes.
func appendString(b []byte, s string) ([]byte, error) {
	if len(s) > math.MaxUint16 {
		return b, ErrTooLarge
	}

	b = appendU16(b, uint16(len(s)))

	return append(b, s...), nil
}
