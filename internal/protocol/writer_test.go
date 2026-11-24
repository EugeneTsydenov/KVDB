package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func TestAppendBytesAndStringBoundaries(t *testing.T) {
	t.Parallel()

	max := bytes.Repeat([]byte{'x'}, math.MaxUint16)
	encoded, err := appendBytes(nil, max)
	if err != nil {
		t.Fatalf("appendBytes at uint16 maximum: %v", err)
	}
	if binary.BigEndian.Uint16(encoded[:2]) != math.MaxUint16 {
		t.Fatal("appendBytes wrote an incorrect length")
	}

	if _, err := appendBytes(nil, append(max, 'x')); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("appendBytes above uint16 maximum: got %v, want ErrTooLarge", err)
	}

	encodedString, err := appendString(nil, string(max))
	if err != nil {
		t.Fatalf("appendString at uint16 maximum: %v", err)
	}
	if binary.BigEndian.Uint16(encodedString[:2]) != math.MaxUint16 {
		t.Fatal("appendString wrote an incorrect length")
	}
	if _, err := appendString(nil, string(append(max, 'x'))); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("appendString above uint16 maximum: got %v, want ErrTooLarge", err)
	}
}

func TestWriterPrimitiveValues(t *testing.T) {
	t.Parallel()

	var got []byte
	got = appendU8(got, 0xAA)
	got = appendU16(got, 0x1234)
	got = appendU64(got, 0x0102030405060708)
	got = appendI64(got, -1)

	want := []byte{
		0xAA,
		0x12, 0x34,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded bytes = % X, want % X", got, want)
	}
}

func TestWriterBytesAndString(t *testing.T) {
	t.Parallel()

	got, err := appendBytes(nil, []byte("key"))
	if err != nil {
		t.Fatalf("appendBytes: %v", err)
	}
	got, err = appendString(got, "value")
	if err != nil {
		t.Fatalf("appendString: %v", err)
	}
	want := []byte{0x00, 0x03, 'k', 'e', 'y', 0x00, 0x05, 'v', 'a', 'l', 'u', 'e'}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded bytes = % X, want % X", got, want)
	}
}
