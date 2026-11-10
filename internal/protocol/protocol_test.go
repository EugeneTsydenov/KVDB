package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func packetForTest(op OpCode, status Status, flags uint8, payload []byte) []byte {
	header := Header{
		Op:         op,
		Status:     status,
		Flags:      flags,
		PayloadLen: uint16(len(payload)),
		RequestID:  42,
	}
	packet := header.appendTo(nil)
	return append(packet, payload...)
}

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

func appendBytesForTest(data []byte) []byte {
	encoded, err := appendBytes(nil, data)
	if err != nil {
		panic(err)
	}
	return encoded
}

func TestResponseRoundTrip(t *testing.T) {
	t.Parallel()

	responses := []Response{
		{Op: OpPing, Status: StatusOK, RequestID: 1},
		{Op: OpGet, Status: StatusOK, RequestID: 2, Value: []byte("Alice")},
		{Op: OpSet, Status: StatusOK, RequestID: 3},
		{Op: OpSetNX, Status: StatusOK, RequestID: 4},
		{Op: OpSetNX, Status: StatusNotSet, RequestID: 5},
		{Op: OpDel, Status: StatusOK, RequestID: 6, Deleted: 1},
		{Op: OpTTL, Status: StatusOK, RequestID: 7, TTLMs: -1},
		{Op: OpTTL, Status: StatusOK, RequestID: 8, TTLMs: 0},
		{Op: OpExpire, Status: StatusOK, RequestID: 9},
		{Op: OpKeys, Status: StatusOK, RequestID: 10, Keys: []string{"user:1", "user:2"}},
		{Op: OpGet, Status: StatusNotFound, RequestID: 11},
		{Op: OpTTL, Status: StatusNotFound, RequestID: 12},
		{Op: OpExpire, Status: StatusNotFound, RequestID: 13},
		{Op: OpSet, Status: StatusBadRequest, RequestID: 14, Message: "bad payload"},
	}

	for _, original := range responses {
		original := original
		t.Run(original.Op.String()+"/"+original.Status.String(), func(t *testing.T) {
			t.Parallel()

			packet, err := EncodeResponse(original)
			if err != nil {
				t.Fatalf("EncodeResponse: %v", err)
			}

			decoded, err := DecodeResponse(packet)
			if err != nil {
				t.Fatalf("DecodeResponse: %v", err)
			}

			if !reflect.DeepEqual(decoded, original) {
				t.Fatalf("round trip mismatch:\n got: %#v\nwant: %#v", decoded, original)
			}

			reencoded, err := EncodeResponse(decoded)
			if err != nil {
				t.Fatalf("EncodeResponse after decode: %v", err)
			}
			if !bytes.Equal(reencoded, packet) {
				t.Fatal("encoding after decoding produced a different packet")
			}
		})
	}
}

func TestEncodeResponseBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("minimum ttl", func(t *testing.T) {
		if _, err := EncodeResponse(Response{Op: OpTTL, Status: StatusOK, TTLMs: -1}); err != nil {
			t.Fatalf("EncodeResponse with ttl -1: %v", err)
		}
	})

	t.Run("ttl below minimum", func(t *testing.T) {
		if _, err := EncodeResponse(Response{Op: OpTTL, Status: StatusOK, TTLMs: -2}); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("got %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("maximum ttl", func(t *testing.T) {
		if _, err := EncodeResponse(Response{Op: OpTTL, Status: StatusOK, TTLMs: MaxTTLMs}); err != nil {
			t.Fatalf("EncodeResponse with MaxTTLMs: %v", err)
		}
	})

	t.Run("ttl above maximum", func(t *testing.T) {
		if _, err := EncodeResponse(Response{Op: OpTTL, Status: StatusOK, TTLMs: MaxTTLMs + 1}); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("got %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("maximum deleted count", func(t *testing.T) {
		if _, err := EncodeResponse(Response{Op: OpDel, Status: StatusOK, Deleted: 1}); err != nil {
			t.Fatalf("EncodeResponse with Deleted=1: %v", err)
		}
	})

	t.Run("deleted count too large", func(t *testing.T) {
		if _, err := EncodeResponse(Response{Op: OpDel, Status: StatusOK, Deleted: 2}); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("got %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("maximum value switches to too large status", func(t *testing.T) {
		packet, err := EncodeResponse(Response{
			Op:     OpGet,
			Status: StatusOK,
			Value:  bytes.Repeat([]byte{'v'}, MaxValueLen),
		})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeResponse(packet)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Status != StatusOK || len(decoded.Value) != MaxValueLen {
			t.Fatalf("unexpected response: %#v", decoded)
		}
	})

	t.Run("value too large switches to too large status", func(t *testing.T) {
		packet, err := EncodeResponse(Response{
			Op:     OpGet,
			Status: StatusOK,
			Value:  bytes.Repeat([]byte{'v'}, MaxValueLen+1),
		})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeResponse(packet)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Status != StatusTooLarge {
			t.Fatalf("status = %v, want TOO_LARGE", decoded.Status)
		}
	})

	t.Run("maximum error message", func(t *testing.T) {
		packet, err := EncodeResponse(Response{
			Op:      OpGet,
			Status:  StatusBadRequest,
			Message: strings.Repeat("e", MaxErrorLen),
		})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeResponse(packet)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Status != StatusBadRequest || len(decoded.Message) != MaxErrorLen {
			t.Fatalf("unexpected response: %#v", decoded)
		}
	})

	t.Run("error message too large switches to too large status", func(t *testing.T) {
		packet, err := EncodeResponse(Response{
			Op:      OpGet,
			Status:  StatusBadRequest,
			Message: strings.Repeat("e", MaxErrorLen+1),
		})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeResponse(packet)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Status != StatusTooLarge {
			t.Fatalf("status = %v, want TOO_LARGE", decoded.Status)
		}
	})

	t.Run("key too large switches to too large status", func(t *testing.T) {
		packet, err := EncodeResponse(Response{
			Op:     OpKeys,
			Status: StatusOK,
			Keys:   []string{strings.Repeat("k", MaxKeyLen+1)},
		})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeResponse(packet)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Status != StatusTooLarge {
			t.Fatalf("status = %v, want TOO_LARGE", decoded.Status)
		}
	})

	t.Run("payload too large switches to too large status", func(t *testing.T) {
		keys := make([]string, 6)
		for i := range keys {
			keys[i] = strings.Repeat("k", MaxKeyLen)
		}
		packet, err := EncodeResponse(Response{Op: OpKeys, Status: StatusOK, Keys: keys})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeResponse(packet)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Status != StatusTooLarge {
			t.Fatalf("status = %v, want TOO_LARGE", decoded.Status)
		}
	})
}

func TestDecodeResponseCopiesInput(t *testing.T) {
	t.Parallel()

	original := Response{Op: OpGet, Status: StatusOK, RequestID: 10, Value: []byte("value")}
	packet, err := EncodeResponse(original)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeResponse(packet)
	if err != nil {
		t.Fatal(err)
	}

	for i := range packet {
		packet[i] = 0
	}

	if !bytes.Equal(decoded.Value, original.Value) {
		t.Fatal("decoded response shares value memory with packet")
	}
}

func TestDecodeResponseMalformedPackets(t *testing.T) {
	t.Parallel()

	truncatedTTL := packetForTest(OpTTL, StatusOK, FlagResponse, []byte{0, 0, 0, 0, 0, 0, 0})
	truncatedGet := packetForTest(OpGet, StatusOK, FlagResponse, []byte{0})
	trailingPing := packetForTest(OpPing, StatusOK, FlagResponse, []byte{1})
	invalidDeleted := packetForTest(OpDel, StatusOK, FlagResponse, appendU64(nil, 2))
	invalidTTL := packetForTest(OpTTL, StatusOK, FlagResponse, appendI64(nil, -2))
	invalidCursor := packetForTest(OpKeys, StatusOK, FlagResponse, appendU64(appendU64(nil, 1), 0))

	truncatedKeyList := appendU64(nil, 0)
	truncatedKeyList = appendU16(truncatedKeyList, 1)
	truncatedKeyList = append(truncatedKeyList, 0)

	oversizedKey := appendU64(nil, 0)
	oversizedKey = appendU16(oversizedKey, 1)
	oversizedKey = appendU16(oversizedKey, MaxKeyLen+1)

	cases := []struct {
		name   string
		packet []byte
		want   error
	}{
		{name: "short header", packet: make([]byte, HeaderSize-1), want: ErrBadLength},
		{name: "request flag", packet: packetForTest(OpPing, StatusOK, 0, nil), want: ErrBadFlags},
		{name: "invalid status", packet: packetForTest(OpPing, Status(0xFF), FlagResponse, nil), want: ErrInvalidArgument},
		{name: "truncated get value", packet: truncatedGet, want: ErrTruncated},
		{name: "trailing ping payload", packet: trailingPing, want: ErrTrailingBytes},
		{name: "truncated ttl", packet: truncatedTTL, want: ErrTruncated},
		{name: "invalid deleted count", packet: invalidDeleted, want: ErrInvalidArgument},
		{name: "invalid ttl", packet: invalidTTL, want: ErrInvalidArgument},
		{name: "nonzero keys cursor", packet: invalidCursor, want: ErrInvalidArgument},
		{name: "truncated keys item", packet: packetForTest(OpKeys, StatusOK, FlagResponse, truncatedKeyList), want: ErrTruncated},
		{name: "keys item too large", packet: packetForTest(OpKeys, StatusOK, FlagResponse, oversizedKey), want: ErrTooLarge},
		{name: "truncated error message", packet: packetForTest(OpGet, StatusBadRequest, FlagResponse, []byte{0}), want: ErrTruncated},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeResponse(tc.packet); !errors.Is(err, tc.want) {
				t.Fatalf("DecodeResponse error = %v, want %v", err, tc.want)
			}
		})
	}
}

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

func TestReaderAndWriter(t *testing.T) {
	t.Parallel()

	raw := appendU16(nil, 0x1234)
	raw = appendU64(raw, 0x0102030405060708)
	raw = appendI64(raw, -1)
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

	if !bytes.Equal(raw[:2], []byte{0x12, 0x34}) {
		t.Fatalf("u16 bytes = % X", raw[:2])
	}
}

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

func TestStatusAndErrorHelpers(t *testing.T) {
	t.Parallel()

	statusCases := []struct {
		err  error
		want Status
	}{
		{err: nil, want: StatusOK},
		{err: ErrBadMagic, want: StatusBadRequest},
		{err: ErrBadLength, want: StatusBadRequest},
		{err: ErrBadFlags, want: StatusBadRequest},
		{err: ErrTruncated, want: StatusBadRequest},
		{err: ErrTrailingBytes, want: StatusBadRequest},
		{err: ErrUnsupportedVersion, want: StatusUnsupportedVersion},
		{err: ErrUnknownOpCode, want: StatusUnknownOpCode},
		{err: ErrTooLarge, want: StatusTooLarge},
		{err: ErrInvalidArgument, want: StatusInvalidArgument},
		{err: errors.New("unexpected"), want: StatusInternalError},
		{err: errors.Join(errors.New("context"), ErrTooLarge), want: StatusTooLarge},
	}

	for _, tc := range statusCases {
		if got := StatusOf(tc.err); got != tc.want {
			t.Errorf("StatusOf(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}

	replyCases := []struct {
		err  error
		want bool
	}{
		{err: nil, want: true},
		{err: ErrBadMagic, want: false},
		{err: ErrBadLength, want: false},
		{err: ErrBadFlags, want: true},
		{err: ErrTruncated, want: true},
		{err: ErrInvalidArgument, want: true},
		{err: errors.Join(errors.New("context"), ErrBadMagic), want: false},
	}

	for _, tc := range replyCases {
		if got := ShouldReply(tc.err); got != tc.want {
			t.Errorf("ShouldReply(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestOpCodeAndStatusValues(t *testing.T) {
	t.Parallel()

	opcodes := []OpCode{OpPing, OpGet, OpSet, OpSetNX, OpDel, OpTTL, OpExpire, OpKeys}
	for _, op := range opcodes {
		if !op.Valid() {
			t.Errorf("%v should be valid", op)
		}
		if op.String() == "" {
			t.Errorf("%d has an empty string representation", op)
		}
	}
	if OpCode(0xFF).Valid() || OpCode(0xFF).String() != "OpCode(0xFF)" {
		t.Error("unknown opcode handling is incorrect")
	}

	statuses := []Status{
		StatusOK,
		StatusNotFound,
		StatusNotSet,
		StatusBadRequest,
		StatusUnknownOpCode,
		StatusUnsupportedVersion,
		StatusTooLarge,
		StatusInvalidArgument,
		StatusInternalError,
	}
	for _, status := range statuses {
		if !status.Valid() {
			t.Errorf("%v should be valid", status)
		}
		if status.String() == "" {
			t.Errorf("%d has an empty string representation", status)
		}
	}
	if Status(0xFF).Valid() || Status(0xFF).String() != "Status(0xFF)" {
		t.Error("unknown status handling is incorrect")
	}
}
