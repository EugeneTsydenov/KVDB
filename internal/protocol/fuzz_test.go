package protocol

import (
	"bytes"
	"reflect"
	"testing"
)

func fuzzRequestPackets() [][]byte {
	requests := []Request{
		{Op: OpPing, RequestID: 1},
		{Op: OpGet, RequestID: 2, Key: []byte("key")},
		{Op: OpSet, RequestID: 3, Key: []byte("key"), Value: []byte("value"), TTLMs: 5000},
		{Op: OpSetNX, RequestID: 4, Key: []byte("lock"), Value: []byte("owner")},
		{Op: OpDel, RequestID: 5, Key: []byte("key")},
		{Op: OpTTL, RequestID: 6, Key: []byte("key")},
		{Op: OpExpire, RequestID: 7, Key: []byte("key"), TTLMs: 1000},
		{Op: OpKeys, RequestID: 8, Pattern: `^user:[0-9]+$`, Cursor: 0, Limit: 10},
	}

	packets := [][]byte{nil, {}, {0}, make([]byte, HeaderSize-1), make([]byte, HeaderSize)}
	for _, req := range requests {
		packet, err := EncodeRequest(req)
		if err == nil {
			packets = append(packets, packet)
		}
	}
	return packets
}

func fuzzResponsePackets() [][]byte {
	responses := []Response{
		{Op: OpPing, Status: StatusOK, RequestID: 1},
		{Op: OpGet, Status: StatusOK, RequestID: 2, Value: []byte("value")},
		{Op: OpSet, Status: StatusOK, RequestID: 3},
		{Op: OpSetNX, Status: StatusNotSet, RequestID: 4},
		{Op: OpDel, Status: StatusOK, RequestID: 5, Deleted: 1},
		{Op: OpTTL, Status: StatusOK, RequestID: 6, TTLMs: -1},
		{Op: OpExpire, Status: StatusNotFound, RequestID: 7},
		{Op: OpKeys, Status: StatusOK, RequestID: 8, Keys: []string{"a", "b"}},
		{Op: OpGet, Status: StatusBadRequest, RequestID: 9, Message: "bad request"},
	}

	packets := [][]byte{nil, {}, {0}, make([]byte, HeaderSize-1), make([]byte, HeaderSize)}
	for _, resp := range responses {
		packet, err := EncodeResponse(resp)
		if err == nil {
			packets = append(packets, packet)
		}
	}
	return packets
}

func nonNilClone(data []byte) []byte {
	if data == nil {
		return []byte{}
	}
	return bytes.Clone(data)
}

func FuzzDecodeRequest(f *testing.F) {
	for _, packet := range fuzzRequestPackets() {
		f.Add(packet)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		_, _ = DecodeRequest(packet)
	})
}

func FuzzRequestRoundTrip(f *testing.F) {
	for _, packet := range fuzzRequestPackets() {
		f.Add(packet)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		first, err := DecodeRequest(packet)
		if err != nil {
			return
		}

		reencoded, err := EncodeRequest(first)
		if err != nil {
			t.Fatalf("EncodeRequest after successful decode: %v", err)
		}

		second, err := DecodeRequest(reencoded)
		if err != nil {
			t.Fatalf("DecodeRequest after re-encode: %v", err)
		}

		if !reflect.DeepEqual(first, second) {
			t.Fatalf("request changed after round trip:\nfirst:  %#v\nsecond: %#v", first, second)
		}
	})
}

func FuzzEncodeRequest(f *testing.F) {
	f.Add(uint8(OpPing), []byte(""), []byte(""), uint64(0), "", uint64(0), uint16(0), uint64(1))
	f.Add(uint8(OpSet), []byte("key"), []byte("value"), uint64(5000), "", uint64(0), uint16(0), uint64(2))
	f.Add(uint8(OpKeys), []byte(""), []byte(""), uint64(0), `^user:[0-9]+$`, uint64(0), uint16(10), uint64(3))

	f.Fuzz(func(
		t *testing.T,
		op uint8,
		key []byte,
		value []byte,
		ttl uint64,
		pattern string,
		cursor uint64,
		limit uint16,
		requestID uint64,
	) {
		req := Request{Op: OpCode(op), RequestID: requestID}

		switch req.Op {
		case OpGet, OpDel, OpTTL:
			req.Key = nonNilClone(key)
		case OpSet, OpSetNX:
			req.Key = nonNilClone(key)
			req.Value = nonNilClone(value)
			req.TTLMs = ttl
		case OpExpire:
			req.Key = nonNilClone(key)
			req.TTLMs = ttl
		case OpKeys:
			req.Pattern = pattern
			req.Cursor = cursor
			req.Limit = limit
		}

		packet, err := EncodeRequest(req)
		if err != nil {
			return
		}

		decoded, err := DecodeRequest(packet)
		if err != nil {
			t.Fatalf("DecodeRequest after successful encode: %v", err)
		}
		if !reflect.DeepEqual(req, decoded) {
			t.Fatalf("request changed after encode/decode:\noriginal: %#v\ndecoded:  %#v", req, decoded)
		}
	})
}

func FuzzDecodeResponse(f *testing.F) {
	for _, packet := range fuzzResponsePackets() {
		f.Add(packet)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		_, _ = DecodeResponse(packet)
	})
}

func FuzzResponseRoundTrip(f *testing.F) {
	for _, packet := range fuzzResponsePackets() {
		f.Add(packet)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		first, err := DecodeResponse(packet)
		if err != nil {
			return
		}

		reencoded, err := EncodeResponse(first)
		if err != nil {
			t.Fatalf("EncodeResponse after successful decode: %v", err)
		}

		second, err := DecodeResponse(reencoded)
		if err != nil {
			t.Fatalf("DecodeResponse after re-encode: %v", err)
		}

		if !reflect.DeepEqual(first, second) {
			t.Fatalf("response changed after round trip:\nfirst:  %#v\nsecond: %#v", first, second)
		}
	})
}

func FuzzEncodeResponse(f *testing.F) {
	f.Add(uint8(OpGet), uint8(StatusOK), []byte("value"), int64(0), uint64(0), "", uint64(1))
	f.Add(uint8(OpTTL), uint8(StatusOK), []byte(""), int64(-1), uint64(0), "", uint64(2))
	f.Add(uint8(OpKeys), uint8(StatusOK), []byte("first"), int64(0), uint64(0), "", uint64(3))
	f.Add(uint8(OpSet), uint8(StatusBadRequest), []byte(""), int64(0), uint64(0), "bad request", uint64(4))

	f.Fuzz(func(
		t *testing.T,
		op uint8,
		status uint8,
		value []byte,
		ttl int64,
		deleted uint64,
		message string,
		requestID uint64,
	) {
		resp := Response{
			Op:        OpCode(op),
			Status:    Status(status),
			RequestID: requestID,
			Message:   message,
		}

		switch resp.Status {
		case StatusOK:
			switch resp.Op {
			case OpGet:
				resp.Value = nonNilClone(value)
			case OpTTL:
				resp.TTLMs = ttl
			case OpDel:
				resp.Deleted = deleted
			case OpKeys:
				resp.Keys = []string{string(value)}
			}
		case StatusNotFound:
			if resp.Op != OpGet && resp.Op != OpTTL && resp.Op != OpExpire {
				resp.Op = OpGet
			}
		case StatusNotSet:
			resp.Op = OpSetNX
			resp.Message = ""
		}

		packet, err := EncodeResponse(resp)
		if err != nil {
			return
		}

		decoded, err := DecodeResponse(packet)
		if err != nil {
			t.Fatalf("DecodeResponse after successful encode: %v", err)
		}

		reencoded, err := EncodeResponse(decoded)
		if err != nil {
			t.Fatalf("EncodeResponse after decode: %v", err)
		}
		if !bytes.Equal(packet, reencoded) {
			t.Fatal("response packet changed after encode/decode/encode")
		}
	})
}
