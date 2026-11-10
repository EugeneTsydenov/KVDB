package protocol

import "encoding/binary"

// reader holds packet bytes and the next position to read.
type reader struct {
	buf []byte
	pos int
}

// newReader returns a reader that starts at the first byte of buf.
func newReader(buf []byte) *reader {
	return &reader{buf: buf}
}

// remaining returns the number of bytes left to read.
func (r *reader) remaining() int {
	return len(r.buf) - r.pos
}

// take returns the next n bytes and moves past them.
// It returns ErrTruncated if n is negative or there are not enough bytes.
func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || r.remaining() < n {
		return nil, ErrTruncated
	}

	data := r.buf[r.pos : r.pos+n]
	r.pos += n

	return data, nil
}

// u16 reads the next 2 bytes and returns a uint16.
// It returns ErrTruncated if there are not enough bytes.
func (r *reader) u16() (uint16, error) {
	//nolint: mnd
	data, err := r.take(2)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint16(data), nil
}

// u64 reads the next 8 bytes and returns a uint64.
// It returns ErrTruncated if there are not enough bytes.
func (r *reader) u64() (uint64, error) {
	//nolint: mnd
	data, err := r.take(8)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(data), nil
}

// i64 reads the next 8 bytes and returns an int64, which can be negative.
// It returns ErrTruncated if there are not enough bytes.
func (r *reader) i64() (int64, error) {
	v, err := r.u64()

	return int64(v), err
}

// bytes reads a 2-byte length, then returns that many bytes.
// The returned slice shares the input buffer.
// It returns ErrTooLarge if the length is above maxLen.
// It returns ErrTruncated if bytes are missing. On error, it does not move.
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

// str reads a 2-byte length, then returns that many bytes as a string.
// It returns the same errors as bytes.
func (r *reader) str(maxLen int) (string, error) {
	data, err := r.bytes(maxLen)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// finish returns nil if all bytes have been read.
// It returns ErrTrailingBytes if any bytes are left.
func (r *reader) finish() error {
	if r.remaining() != 0 {
		return ErrTrailingBytes
	}

	return nil
}
