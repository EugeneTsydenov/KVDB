package protocol

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

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
