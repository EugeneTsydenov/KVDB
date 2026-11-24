package protocol

import (
	"encoding/binary"
	"math"
)

// appendU8 adds one byte to b and returns the new slice.
func appendU8(b []byte, v uint8) []byte {
	return append(b, v)
}

// appendU16 adds v to b as 2 bytes and returns the new slice.
func appendU16(b []byte, v uint16) []byte {
	return binary.BigEndian.AppendUint16(b, v)
}

// appendU64 adds v to b as 8 bytes and returns the new slice.
func appendU64(b []byte, v uint64) []byte {
	return binary.BigEndian.AppendUint64(b, v)
}

// appendI64 adds v to b as 8 bytes and returns the new slice.
// v can be negative.
func appendI64(b []byte, v int64) []byte {
	return binary.BigEndian.AppendUint64(b, uint64(v))
}

// appendBytes adds a 2-byte length and the data to b, then returns the new slice.
// If data is longer than 65535 bytes, it returns b unchanged and ErrTooLarge.
func appendBytes(b []byte, data []byte) ([]byte, error) {
	if len(data) > math.MaxUint16 {
		return b, ErrTooLarge
	}

	b = appendU16(b, uint16(len(data)))

	return append(b, data...), nil
}

// appendString adds a 2-byte length and the string bytes to b, then returns the new slice.
// If the string is longer than 65535 bytes, it returns b unchanged and ErrTooLarge.
func appendString(b []byte, s string) ([]byte, error) {
	if len(s) > math.MaxUint16 {
		return b, ErrTooLarge
	}

	b = appendU16(b, uint16(len(s)))

	return append(b, s...), nil
}
