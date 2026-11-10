package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	t.Parallel()

	requests := []Request{
		{Op: OpPing, RequestID: 1},
		{Op: OpGet, RequestID: 2, Key: []byte("user:1")},
		{Op: OpSet, RequestID: 3, Key: []byte("user:1"), Value: []byte("Alice"), TTLMs: 5000},
		{Op: OpSetNX, RequestID: 4, Key: []byte("lock"), Value: []byte("owner"), TTLMs: 1000},
		{Op: OpDel, RequestID: 5, Key: []byte("user:1")},
		{Op: OpTTL, RequestID: 6, Key: []byte("user:1")},
		{Op: OpExpire, RequestID: 7, Key: []byte("user:1"), TTLMs: 9000},
		{Op: OpKeys, RequestID: 8, Pattern: `^user:[0-9]+$`, Cursor: 17, Limit: 25},
	}

	for _, original := range requests {
		original := original
		t.Run(original.Op.String(), func(t *testing.T) {
			t.Parallel()

			packet, err := EncodeRequest(original)
			if err != nil {
				t.Fatalf("EncodeRequest: %v", err)
			}

			decoded, err := DecodeRequest(packet)
			if err != nil {
				t.Fatalf("DecodeRequest: %v", err)
			}

			if !reflect.DeepEqual(decoded, original) {
				t.Fatalf("round trip mismatch:\n got: %#v\nwant: %#v", decoded, original)
			}

			reencoded, err := EncodeRequest(decoded)
			if err != nil {
				t.Fatalf("EncodeRequest after decode: %v", err)
			}
			if !bytes.Equal(reencoded, packet) {
				t.Fatal("encoding after decoding produced a different packet")
			}
		})
	}
}

func TestEncodeRequestBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("maximum key length", func(t *testing.T) {
		req := Request{Op: OpGet, Key: bytes.Repeat([]byte{'k'}, MaxKeyLen)}
		if _, err := EncodeRequest(req); err != nil {
			t.Fatalf("EncodeRequest with MaxKeyLen: %v", err)
		}
	})

	t.Run("key too large", func(t *testing.T) {
		req := Request{Op: OpGet, Key: bytes.Repeat([]byte{'k'}, MaxKeyLen+1)}
		if _, err := EncodeRequest(req); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("got %v, want ErrTooLarge", err)
		}
	})

	t.Run("maximum value length", func(t *testing.T) {
		req := Request{
			Op:    OpSet,
			Key:   []byte("key"),
			Value: bytes.Repeat([]byte{'v'}, MaxValueLen),
		}
		if _, err := EncodeRequest(req); err != nil {
			t.Fatalf("EncodeRequest with MaxValueLen: %v", err)
		}
	})

	t.Run("value too large", func(t *testing.T) {
		req := Request{
			Op:    OpSet,
			Key:   []byte("key"),
			Value: bytes.Repeat([]byte{'v'}, MaxValueLen+1),
		}
		if _, err := EncodeRequest(req); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("got %v, want ErrTooLarge", err)
		}
	})

	t.Run("maximum pattern length", func(t *testing.T) {
		req := Request{Op: OpKeys, Pattern: strings.Repeat("a", MaxPatternLen)}
		if _, err := EncodeRequest(req); err != nil {
			t.Fatalf("EncodeRequest with MaxPatternLen: %v", err)
		}
	})

	t.Run("pattern too large", func(t *testing.T) {
		req := Request{Op: OpKeys, Pattern: strings.Repeat("a", MaxPatternLen+1)}
		if _, err := EncodeRequest(req); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("got %v, want ErrTooLarge", err)
		}
	})

	t.Run("maximum ttl", func(t *testing.T) {
		req := Request{Op: OpSet, Key: []byte("key"), TTLMs: MaxTTLMs}
		if _, err := EncodeRequest(req); err != nil {
			t.Fatalf("EncodeRequest with MaxTTLMs: %v", err)
		}
	})

	t.Run("ttl too large", func(t *testing.T) {
		req := Request{Op: OpSet, Key: []byte("key"), TTLMs: MaxTTLMs + 1}
		if _, err := EncodeRequest(req); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("got %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("expire zero ttl", func(t *testing.T) {
		req := Request{Op: OpExpire, Key: []byte("key")}
		if _, err := EncodeRequest(req); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("got %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("expire maximum ttl", func(t *testing.T) {
		req := Request{Op: OpExpire, Key: []byte("key"), TTLMs: MaxTTLMs}
		if _, err := EncodeRequest(req); err != nil {
			t.Fatalf("EncodeRequest with MaxTTLMs: %v", err)
		}
	})

	t.Run("unknown opcode", func(t *testing.T) {
		req := Request{Op: OpCode(0xFF)}
		if _, err := EncodeRequest(req); !errors.Is(err, ErrUnknownOpCode) {
			t.Fatalf("got %v, want ErrUnknownOpCode", err)
		}
	})
}

func TestDecodeRequestCopiesInput(t *testing.T) {
	t.Parallel()

	original := Request{Op: OpSet, RequestID: 9, Key: []byte("key"), Value: []byte("value"), TTLMs: 100}
	packet, err := EncodeRequest(original)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeRequest(packet)
	if err != nil {
		t.Fatal(err)
	}

	for i := range packet {
		packet[i] = 0
	}

	if !bytes.Equal(decoded.Key, original.Key) || !bytes.Equal(decoded.Value, original.Value) {
		t.Fatal("decoded request shares key or value memory with packet")
	}
}

func TestDecodeRequestMalformedPackets(t *testing.T) {
	t.Parallel()

	validGetPayload, err := appendBytes(nil, []byte("key"))
	if err != nil {
		t.Fatal(err)
	}

	invalidRegexPayload, err := appendString(nil, "[")
	if err != nil {
		t.Fatal(err)
	}
	invalidRegexPayload = appendU64(invalidRegexPayload, 0)
	invalidRegexPayload = appendU16(invalidRegexPayload, 0)

	badLength := packetForTest(OpGet, StatusOK, 0, validGetPayload)
	binary.BigEndian.PutUint16(badLength[6:8], uint16(len(validGetPayload)+1))

	badMagic := packetForTest(OpPing, StatusOK, 0, nil)
	badMagic[0] = 0

	badVersion := packetForTest(OpPing, StatusOK, 0, nil)
	badVersion[2]++

	cases := []struct {
		name   string
		packet []byte
		want   error
	}{
		{name: "short header", packet: make([]byte, HeaderSize-1), want: ErrBadLength},
		{name: "bad magic", packet: badMagic, want: ErrBadMagic},
		{name: "unsupported version", packet: badVersion, want: ErrUnsupportedVersion},
		{name: "payload length mismatch", packet: badLength, want: ErrBadLength},
		{name: "request flags set", packet: packetForTest(OpPing, StatusOK, FlagResponse, nil), want: ErrBadFlags},
		{name: "nonzero request status", packet: packetForTest(OpPing, StatusNotFound, 0, nil), want: ErrInvalidArgument},
		{name: "unknown opcode", packet: packetForTest(OpCode(0xFF), StatusOK, 0, nil), want: ErrUnknownOpCode},
		{name: "truncated key length", packet: packetForTest(OpGet, StatusOK, 0, []byte{0}), want: ErrTruncated},
		{name: "trailing ping payload", packet: packetForTest(OpPing, StatusOK, 0, []byte{1}), want: ErrTrailingBytes},
		{name: "truncated set ttl", packet: packetForTest(OpSet, StatusOK, 0, append(appendBytesForTest([]byte("key")), appendBytesForTest([]byte("value"))...)), want: ErrTruncated},
		{name: "invalid keys regex", packet: packetForTest(OpKeys, StatusOK, 0, invalidRegexPayload), want: ErrInvalidArgument},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeRequest(tc.packet); !errors.Is(err, tc.want) {
				t.Fatalf("DecodeRequest error = %v, want %v", err, tc.want)
			}
		})
	}

	tooLarge := make([]byte, MaxPacketSize+1)
	if _, err := DecodeRequest(tooLarge); !errors.Is(err, ErrBadLength) {
		t.Fatalf("oversized packet: got %v, want ErrBadLength", err)
	}
}
