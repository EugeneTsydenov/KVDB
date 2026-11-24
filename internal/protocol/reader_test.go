package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func TestReaderErrorsAndPosition(t *testing.T) {
	t.Parallel()

	t.Run("negative take", func(t *testing.T) {
		r := newReader([]byte{1, 2, 3})
		if _, err := r.take(-1); !errors.Is(err, ErrTruncated) {
			t.Fatalf("got %v, want ErrTruncated", err)
		}
		if r.pos != 0 {
			t.Fatalf("position = %d, want 0", r.pos)
		}
	})

	t.Run("truncated take", func(t *testing.T) {
		r := newReader([]byte{1, 2})
		if _, err := r.take(3); !errors.Is(err, ErrTruncated) {
			t.Fatalf("got %v, want ErrTruncated", err)
		}
		if r.pos != 0 {
			t.Fatalf("position = %d, want 0", r.pos)
		}
	})

	t.Run("bytes too large does not advance", func(t *testing.T) {
		r := newReader(appendU16(nil, 3))
		if _, err := r.bytes(2); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("got %v, want ErrTooLarge", err)
		}
		if r.pos != 0 {
			t.Fatalf("position = %d, want 0", r.pos)
		}
	})

	t.Run("truncated bytes does not advance", func(t *testing.T) {
		r := newReader([]byte{0, 3, 'a'})
		if _, err := r.bytes(3); !errors.Is(err, ErrTruncated) {
			t.Fatalf("got %v, want ErrTruncated", err)
		}
		if r.pos != 0 {
			t.Fatalf("position = %d, want 0", r.pos)
		}
	})

	t.Run("trailing bytes", func(t *testing.T) {
		r := newReader([]byte{1})
		if err := r.finish(); !errors.Is(err, ErrTrailingBytes) {
			t.Fatalf("got %v, want ErrTrailingBytes", err)
		}
	})

	t.Run("uint16 truncation", func(t *testing.T) {
		r := newReader([]byte{1})
		if _, err := r.u16(); !errors.Is(err, ErrTruncated) {
			t.Fatalf("got %v, want ErrTruncated", err)
		}
	})

	t.Run("uint64 truncation", func(t *testing.T) {
		r := newReader(make([]byte, 7))
		if _, err := r.u64(); !errors.Is(err, ErrTruncated) {
			t.Fatalf("got %v, want ErrTruncated", err)
		}
	})
}

func TestReaderPrimitiveValues(t *testing.T) {
	t.Parallel()

	raw := []byte{
		0x12, 0x34,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}
	r := newReader(raw)

	u16, err := r.u16()
	if err != nil || u16 != 0x1234 {
		t.Fatalf("u16 = %#x, %v", u16, err)
	}
	u64, err := r.u64()
	if err != nil || u64 != 0x0102030405060708 {
		t.Fatalf("u64 = %#x, %v", u64, err)
	}
	i64, err := r.i64()
	if err != nil || i64 != -1 {
		t.Fatalf("i64 = %d, %v", i64, err)
	}
	if err := r.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
}

func TestReaderBytesAndString(t *testing.T) {
	t.Parallel()

	r := newReader([]byte{0x00, 0x03, 'k', 'e', 'y', 0x00, 0x02, 'o', 'k'})

	gotString, err := r.str(3)
	if err != nil || gotString != "key" {
		t.Fatalf("str = %q, %v; want key", gotString, err)
	}
	gotBytes, err := r.bytes(2)
	if err != nil || !bytes.Equal(gotBytes, []byte("ok")) {
		t.Fatalf("bytes = %q, %v; want ok", gotBytes, err)
	}
	if err := r.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
}
