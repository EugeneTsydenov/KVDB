package protocol

import "encoding/binary"

// Reads values from a buffer one by one.
type reader struct {
	buf []byte
	pos int
}

func newReader(buf []byte) *reader {
	return &reader{buf: buf}
}

// Returns the number of unread bytes.
func (r *reader) remaining() int {
	return len(r.buf) - r.pos
}

// Reads the next n bytes and moves the position forward.
// Returns ErrTruncated if there are not enough bytes.
func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || r.remaining() < n {
		return nil, ErrTruncated
	}

	data := r.buf[r.pos : r.pos+n]
	r.pos += n

	return data, nil
}

// Reads a single byte.
func (r *reader) u8() (uint8, error) {
	data, err := r.take(1)
	if err != nil {
		return 0, err
	}

	return data[0], nil
}

// Reads a 2-byte unsigned integer in big-endian order.
func (r *reader) u16() (uint16, error) {
	//nolint: mnd
	data, err := r.take(2)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint16(data), nil
}

// Reads an 8-byte unsigned integer in big-endian order.
func (r *reader) u64() (uint64, error) {
	//nolint: mnd
	data, err := r.take(8)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(data), nil
}

// Reads an 8-byte signed integer.
func (r *reader) i64() (int64, error) {
	v, err := r.u64()

	return int64(v), err
}

// Reads a u16 length followed by the data.
// Returns ErrTooLarge if the length exceeds maxLen.
// Returns ErrTruncated if the buffer does not contain all the data.
// On error, the position stays unchanged.
func (r *reader) bytes(maxLen int) ([]byte, error) {
	start := r.pos

	n, err := r.u16()
	if err != nil {
		return nil, err
	}

	if int(n) > maxLen {
		r.pos = start

		return nil, ErrTooLarge
	}

	data, err := r.take(int(n))
	if err != nil {
		r.pos = start

		return nil, err
	}

	return data, nil
}

// Reads a string using the same format as bytes.
func (r *reader) str(maxLen int) (string, error) {
	data, err := r.bytes(maxLen)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Checks that all bytes have been read.
func (r *reader) finish() error {
	if r.remaining() != 0 {
		return ErrTrailingBytes
	}

	return nil
}
