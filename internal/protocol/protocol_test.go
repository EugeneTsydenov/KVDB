package protocol_test

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"kvdb/internal/protocol"
	"kvdb/internal/protocol/pb"

	"google.golang.org/protobuf/proto"
)

func header(op protocol.OpCode) protocol.Header {
	return protocol.Header{Op: op, ClientID: 11, RequestID: 22, Sequence: 33}
}

func packet(op pb.Operation, req *pb.Request) *pb.Packet {
	return &pb.Packet{Version: 1, ClientId: 11, RequestId: 22, Sequence: 33,
		Op: op, Body: &pb.Packet_Request{Request: req}}
}

func TestEnumNumbers(t *testing.T) {
	ops := []protocol.OpCode{protocol.OpPing, protocol.OpGet, protocol.OpSet, protocol.OpSetNX,
		protocol.OpDel, protocol.OpTTL, protocol.OpExpire, protocol.OpKeys}
	for i, op := range ops {
		if int(op) != i+1 || int(op.ToPbOperation()) != i+1 {
			t.Errorf("%s has incorrect code", op)
		}
		got, err := protocol.OpCodeFromPb(op.ToPbOperation())
		if err != nil || got != op {
			t.Fatalf("operation conversion: %v, %v", got, err)
		}
	}
	statuses := map[protocol.Status]int{protocol.StatusOK: 1, protocol.StatusNotFound: 2,
		protocol.StatusNotSet: 3, protocol.StatusBadRequest: 16, protocol.StatusUnknownOpCode: 17,
		protocol.StatusUnsupportedVersion: 18, protocol.StatusTooLarge: 19, protocol.StatusInvalidArgument: 20,
		protocol.StatusRequestConflict: 21, protocol.StatusServerBusy: 22, protocol.StatusInternalError: 127}
	for s, n := range statuses {
		if int(s) != n || int(s.ToPbStatus()) != n {
			t.Errorf("%s has incorrect code", s)
		}
		got, err := protocol.StatusFromPb(s.ToPbStatus())
		if err != nil || got != s {
			t.Fatalf("status conversion: %v, %v", got, err)
		}
	}
	if protocol.OpUnspecified.Valid() || protocol.StatusUnspecified.Valid() {
		t.Fatal("zero enums must be invalid")
	}
}

func TestRequestRoundTrip(t *testing.T) {
	cases := []protocol.Request{
		{Header: header(protocol.OpPing)},
		{Header: header(protocol.OpGet), Key: []byte{0, 255}},
		{Header: header(protocol.OpSet), Key: bytes.Repeat([]byte{255}, protocol.MaxKeyLen),
			Value: bytes.Repeat([]byte{254}, protocol.MaxValueLen), TTLMs: protocol.MaxTTLMs},
		{Header: header(protocol.OpSetNX), Key: []byte("k")},
		{Header: header(protocol.OpDel), Key: []byte("k")},
		{Header: header(protocol.OpTTL), Key: []byte("k")},
		{Header: header(protocol.OpExpire), Key: []byte("k"), TTLMs: 1},
		{Header: header(protocol.OpKeys), Pattern: "^user:.*$"},
		{Header: header(protocol.OpKeys)},
	}
	for _, req := range cases {
		t.Run(req.Header.Op.String(), func(t *testing.T) {
			data, err := protocol.EncodeRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			wire := new(pb.Packet)
			if err := proto.Unmarshal(data, wire); err != nil {
				t.Fatal(err)
			}
			if wire.GetRequest() == nil || wire.GetOp() != req.Header.Op.ToPbOperation() || wire.GetVersion() != 1 {
				t.Fatal("incorrect protobuf envelope")
			}
			got, err := protocol.DecodeRequest(data)
			if err != nil || !reflect.DeepEqual(got, req) {
				t.Fatalf("round trip: %#v, %v", got, err)
			}
		})
	}
}

func TestInvalidRequests(t *testing.T) {
	cases := []struct {
		name string
		op   pb.Operation
		req  *pb.Request
		want error
	}{
		{"unknown", 12345, &pb.Request{}, protocol.ErrUnknownOpcode},
		{"unspecified", 0, &pb.Request{}, protocol.ErrUnknownOpcode},
		{"empty key", pb.Operation_OPERATION_GET, &pb.Request{}, protocol.ErrInvalidArgument},
		{"long key", pb.Operation_OPERATION_GET, &pb.Request{Key: make([]byte, 257)}, protocol.ErrPacketTooLarge},
		{"long value", pb.Operation_OPERATION_SET, &pb.Request{Key: []byte("k"), Value: make([]byte, 1025)}, protocol.ErrPacketTooLarge},
		{"long pattern", pb.Operation_OPERATION_KEYS, &pb.Request{Pattern: strings.Repeat("a", 257)}, protocol.ErrPacketTooLarge},
		{"ttl overflow", pb.Operation_OPERATION_SET, &pb.Request{Key: []byte("k"), TtlMs: protocol.MaxTTLMs + 1}, protocol.ErrInvalidArgument},
		{"zero expire", pb.Operation_OPERATION_EXPIRE, &pb.Request{Key: []byte("k")}, protocol.ErrInvalidArgument},
		{"ping arguments", pb.Operation_OPERATION_PING, &pb.Request{Key: []byte("k")}, protocol.ErrInvalidPacket},
		{"get value", pb.Operation_OPERATION_GET, &pb.Request{Key: []byte("k"), Value: []byte("v")}, protocol.ErrInvalidPacket},
		{"set pattern", pb.Operation_OPERATION_SET, &pb.Request{Key: []byte("k"), Pattern: "x"}, protocol.ErrInvalidPacket},
		{"keys cursor", pb.Operation_OPERATION_KEYS, &pb.Request{Cursor: 1}, protocol.ErrInvalidArgument},
		{"keys limit", pb.Operation_OPERATION_KEYS, &pb.Request{Limit: 1}, protocol.ErrInvalidArgument},
		{"bad regex", pb.Operation_OPERATION_KEYS, &pb.Request{Pattern: "["}, protocol.ErrInvalidArgument},
		{"bad utf8", pb.Operation_OPERATION_KEYS, &pb.Request{Pattern: string([]byte{255})}, protocol.ErrInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := protocol.Validate(packet(tc.op, tc.req)); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestEnvelopeValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*pb.Packet)
		want   error
	}{
		{"version", func(p *pb.Packet) { p.Version = 2 }, protocol.ErrUnsupportedVersion},
		{"client", func(p *pb.Packet) { p.ClientId = 0 }, protocol.ErrInvalidPacket},
		{"request", func(p *pb.Packet) { p.RequestId = 0 }, protocol.ErrInvalidPacket},
		{"sequence", func(p *pb.Packet) { p.Sequence = 0 }, protocol.ErrInvalidPacket},
		{"missing body", func(p *pb.Packet) { p.Body = nil }, protocol.ErrInvalidPacket},
		{"nil request", func(p *pb.Packet) { p.Body = &pb.Packet_Request{} }, protocol.ErrInvalidPacket},
		{"typed nil request", func(p *pb.Packet) { p.Body = (*pb.Packet_Request)(nil) }, protocol.ErrInvalidPacket},
		{"typed nil response", func(p *pb.Packet) { p.Body = (*pb.Packet_Response)(nil) }, protocol.ErrInvalidPacket},
		{"typed nil ack", func(p *pb.Packet) { p.Body = (*pb.Packet_Ack)(nil) }, protocol.ErrInvalidPacket},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := packet(pb.Operation_OPERATION_PING, &pb.Request{})
			tc.change(p)
			if err := protocol.Validate(p); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	if err := protocol.Validate(nil); !errors.Is(err, protocol.ErrInvalidPacket) {
		t.Fatal(err)
	}
	for _, data := range [][]byte{nil, {0xff}, make([]byte, protocol.MaxPacketSize+1)} {
		if _, err := protocol.Decode(data); err == nil {
			t.Fatal("accepted invalid datagram")
		}
	}
}

func TestResponseRules(t *testing.T) {
	cases := []struct {
		name string
		op   pb.Operation
		res  *pb.Response
		want error
	}{
		{"empty get", pb.Operation_OPERATION_GET, &pb.Response{Status: pb.Status_STATUS_OK}, nil},
		{"no expiration", pb.Operation_OPERATION_TTL, &pb.Response{Status: pb.Status_STATUS_OK, TtlMs: -1}, nil},
		{"sub millisecond", pb.Operation_OPERATION_TTL, &pb.Response{Status: pb.Status_STATUS_OK}, nil},
		{"bad ttl", pb.Operation_OPERATION_TTL, &pb.Response{Status: pb.Status_STATUS_OK, TtlMs: -2}, protocol.ErrInvalidArgument},
		{"long ttl", pb.Operation_OPERATION_TTL, &pb.Response{Status: pb.Status_STATUS_OK, TtlMs: int64(protocol.MaxTTLMs) + 1}, protocol.ErrInvalidArgument},
		{"delete count", pb.Operation_OPERATION_DEL, &pb.Response{Status: pb.Status_STATUS_OK, Deleted: 2}, protocol.ErrInvalidPacket},
		{"unspecified status", pb.Operation_OPERATION_PING, &pb.Response{}, protocol.ErrInvalidPacket},
		{"unknown status", pb.Operation_OPERATION_PING, &pb.Response{Status: 99}, protocol.ErrInvalidPacket},
		{"not found set", pb.Operation_OPERATION_SET, &pb.Response{Status: pb.Status_STATUS_NOT_FOUND}, protocol.ErrInvalidPacket},
		{"not set get", pb.Operation_OPERATION_GET, &pb.Response{Status: pb.Status_STATUS_NOT_SET}, protocol.ErrInvalidPacket},
		{"error data", pb.Operation_OPERATION_GET, &pb.Response{Status: pb.Status_STATUS_BAD_REQUEST, Message: "bad", Value: []byte("v")}, protocol.ErrInvalidPacket},
		{"error missing message", pb.Operation_OPERATION_GET, &pb.Response{Status: pb.Status_STATUS_BAD_REQUEST}, protocol.ErrInvalidPacket},
		{"long message", pb.Operation_OPERATION_GET, &pb.Response{Status: pb.Status_STATUS_BAD_REQUEST, Message: strings.Repeat("a", 257)}, protocol.ErrPacketTooLarge},
		{"success message", pb.Operation_OPERATION_PING, &pb.Response{Status: pb.Status_STATUS_OK, Message: "bad"}, protocol.ErrInvalidPacket},
		{"unsorted keys", pb.Operation_OPERATION_KEYS, &pb.Response{Status: pb.Status_STATUS_OK, Keys: [][]byte{[]byte("b"), []byte("a")}}, protocol.ErrInvalidPacket},
		{"binary keys", pb.Operation_OPERATION_KEYS, &pb.Response{Status: pb.Status_STATUS_OK, Keys: [][]byte{{0}, {255}}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := packet(tc.op, nil)
			p.Body = &pb.Packet_Response{Response: tc.res}
			if err := protocol.Validate(p); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestUnknownOperationResponse(t *testing.T) {
	for _, op := range []protocol.OpCode{0, 12345, -1} {
		for _, status := range []protocol.Status{protocol.StatusUnknownOpCode, protocol.StatusUnsupportedVersion} {
			want := protocol.Response{Header: header(op), Status: status, Message: "unsupported"}
			data, err := protocol.EncodeResponse(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := protocol.DecodeResponse(data)
			if err != nil || got.Header != want.Header || got.Status != status {
				t.Fatalf("got %#v, %v", got, err)
			}
		}
	}
}

func TestResponseSortingAndFallback(t *testing.T) {
	keys := []string{"z", string([]byte{255}), "a"}
	data, err := protocol.EncodeResponse(protocol.Response{Header: header(protocol.OpKeys), Status: protocol.StatusOK, Keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	got, err := protocol.DecodeResponse(data)
	if err != nil || !reflect.DeepEqual(got.Keys, []string{"a", "z", string([]byte{255})}) {
		t.Fatalf("got %v, %v", got.Keys, err)
	}
	if keys[0] != "z" {
		t.Fatal("mutated caller keys")
	}
	data, err = protocol.EncodeResponse(protocol.Response{Header: header(protocol.OpKeys), Status: protocol.StatusOK,
		Keys: []string{strings.Repeat("a", 256), strings.Repeat("b", 256), strings.Repeat("c", 256), strings.Repeat("d", 256), strings.Repeat("e", 256), strings.Repeat("f", 256)}})
	if err != nil {
		t.Fatal(err)
	}
	got, err = protocol.DecodeResponse(data)
	if err != nil || got.Status != protocol.StatusTooLarge || got.Header != header(protocol.OpKeys) || len(got.Keys) != 0 || len(data) > protocol.MaxPacketSize {
		t.Fatalf("bad fallback: %#v, %v", got, err)
	}
}

func TestAck(t *testing.T) {
	h := header(protocol.OpSet)
	data, err := protocol.EncodeAck(h)
	if err != nil {
		t.Fatal(err)
	}
	got, err := protocol.DecodeAck(data)
	if err != nil || got != h {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := protocol.DecodeRequest(data); err == nil {
		t.Fatal("ACK accepted as request")
	}
	if _, err := protocol.DecodeResponse(data); err == nil {
		t.Fatal("ACK accepted as response")
	}
	if _, err := protocol.DecodeHeader(data); err == nil {
		t.Fatal("ACK accepted for error reply")
	}
}

func TestPacketSizeBoundary(t *testing.T) {
	// Unknown protobuf fields count toward the datagram limit as well.
	p := packet(pb.Operation_OPERATION_PING, &pb.Request{})
	base := proto.Size(p)
	for _, size := range []int{protocol.MaxPacketSize, protocol.MaxPacketSize + 1} {
		// Field 100, length-delimited; tag and length occupy four bytes.
		n := size - base - 4
		unknown := []byte{0xa2, 0x06, byte(n&127) | 0x80, byte(n >> 7)}
		unknown = append(unknown, make([]byte, n)...)
		p.ProtoReflect().SetUnknown(unknown)
		data, err := proto.Marshal(p)
		if err != nil || len(data) != size {
			t.Fatalf("test packet size %d: %v", len(data), err)
		}
		_, encErr := protocol.Encode(p)
		_, decErr := protocol.Decode(data)
		if size == protocol.MaxPacketSize {
			if encErr != nil || decErr != nil {
				t.Fatalf("boundary rejected: %v, %v", encErr, decErr)
			}
		} else if !errors.Is(encErr, protocol.ErrPacketTooLarge) || !errors.Is(decErr, protocol.ErrPacketTooLarge) {
			t.Fatalf("oversize accepted: %v, %v", encErr, decErr)
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{0xff})
	data, err := protocol.EncodeRequest(protocol.Request{Header: header(protocol.OpPing)})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := protocol.Decode(data)
		if err == nil {
			if _, err := protocol.Encode(p); err != nil {
				t.Fatalf("decoded packet cannot be encoded: %v", err)
			}
		}
	})
}
