package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

func TestDecodeHeader(t *testing.T) {
	t.Parallel()

	original := Header{
		Op:         OpKeys,
		Status:     StatusNotFound,
		Flags:      FlagResponse,
		PayloadLen: 0,
		RequestID:  0x0102030405060708,
	}
	packet := original.appendTo(nil)

	decoded, err := decodeHeader(packet)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("decoded header = %#v, want %#v", decoded, original)
	}
	if !bytes.Equal(packet[:2], []byte{0x4B, 0x56}) {
		t.Fatalf("magic bytes = % X, want 4B 56", packet[:2])
	}
	if binary.BigEndian.Uint64(packet[8:16]) != original.RequestID {
		t.Fatal("request ID is not encoded in big-endian order")
	}

	if _, err := decodeHeader(make([]byte, MaxPacketSize+1)); !errors.Is(err, ErrBadLength) {
		t.Fatalf("oversized packet: got %v, want ErrBadLength", err)
	}
}

func TestHeaderAppendToEncoding(t *testing.T) {
	t.Parallel()

	header := Header{
		Op:         OpSet,
		Status:     StatusNotFound,
		Flags:      FlagResponse,
		PayloadLen: 0x1234,
		RequestID:  0x0102030405060708,
	}

	got := header.appendTo(nil)
	want := []byte{
		0x4B, 0x56, 0x01, 0x03, 0x01, 0x01, 0x12, 0x34,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("header bytes = % X, want % X", got, want)
	}
}

func TestDecodeHeaderRejectsMalformedPackets(t *testing.T) {
	t.Parallel()

	valid := packetForTest(OpPing, StatusOK, 0, nil)
	badMagic := append([]byte(nil), valid...)
	badMagic[0] = 0
	badVersion := append([]byte(nil), valid...)
	badVersion[2]++
	badPayloadLength := append([]byte(nil), valid...)
	binary.BigEndian.PutUint16(badPayloadLength[6:8], 1)

	cases := []struct {
		name   string
		packet []byte
		want   error
	}{
		{name: "short header", packet: make([]byte, HeaderSize-1), want: ErrBadLength},
		{name: "oversized packet", packet: make([]byte, MaxPacketSize+1), want: ErrBadLength},
		{name: "bad magic", packet: badMagic, want: ErrBadMagic},
		{name: "unsupported version", packet: badVersion, want: ErrUnsupportedVersion},
		{name: "payload length mismatch", packet: badPayloadLength, want: ErrBadLength},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := decodeHeader(tc.packet); !errors.Is(err, tc.want) {
				t.Fatalf("decodeHeader error = %v, want %v", err, tc.want)
			}
		})
	}
}
