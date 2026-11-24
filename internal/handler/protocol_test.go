package handler_test

import (
	"bytes"
	"testing"
	"time"

	"kvdb/internal/handler"
	"kvdb/internal/protocol"
	"kvdb/internal/protocol/pb"
	"kvdb/internal/storage"

	"google.golang.org/protobuf/proto"
)

type ttlStore struct {
	handler.Store
	remaining time.Duration
}

func (s ttlStore) TTL(string) (time.Duration, error) {
	return s.remaining, nil
}

func TestTTLRoundsDownToMilliseconds(t *testing.T) {
	for _, tc := range []struct {
		name      string
		remaining time.Duration
		want      int64
	}{
		{"below millisecond", time.Millisecond - time.Nanosecond, 0},
		{"whole millisecond", time.Millisecond, 1},
		{"fractional milliseconds", 1500 * time.Microsecond, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := handler.New(ttlStore{remaining: tc.remaining})
			if err != nil {
				t.Fatal(err)
			}
			data, err := protocol.EncodeRequest(protocol.Request{
				Header: protocol.Header{Op: protocol.OpTTL, ClientID: 1, RequestID: 2, Sequence: 3},
				Key:    []byte("k"),
			})
			if err != nil {
				t.Fatal(err)
			}
			reply, err := h.Handle(data)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := protocol.DecodeResponse(reply)
			if err != nil {
				t.Fatal(err)
			}
			if resp.Status != protocol.StatusOK || resp.TTLMs != tc.want {
				t.Fatalf("TTL(%v): status=%s, ttl_ms=%d; want OK, %d", tc.remaining, resp.Status, resp.TTLMs, tc.want)
			}
		})
	}
}

func TestInvalidRequestHandling(t *testing.T) {
	store := storage.NewMemoryStore()
	h, err := handler.New(store)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func(*pb.Packet)
		status protocol.Status
	}{
		{"unsupported version", func(p *pb.Packet) { p.Version = 2 }, protocol.StatusUnsupportedVersion},
		{"unknown operation", func(p *pb.Packet) { p.Op = 12345 }, protocol.StatusUnknownOpCode},
		{"unspecified operation", func(p *pb.Packet) { p.Op = 0 }, protocol.StatusUnknownOpCode},
		{"invalid arguments", func(p *pb.Packet) { p.GetRequest().TtlMs = protocol.MaxTTLMs + 1 }, protocol.StatusInvalidArgument},
		{"unused fields", func(p *pb.Packet) { p.GetRequest().Pattern = "x" }, protocol.StatusBadRequest},
		{"long key", func(p *pb.Packet) { p.GetRequest().Key = make([]byte, 257) }, protocol.StatusTooLarge},
		{"zero client", func(p *pb.Packet) { p.ClientId = 0 }, 0},
		{"zero request", func(p *pb.Packet) { p.RequestId = 0 }, 0},
		{"zero sequence", func(p *pb.Packet) { p.Sequence = 0 }, 0},
		{"missing body", func(p *pb.Packet) { p.Body = nil }, 0},
		{"ack", func(p *pb.Packet) { p.Body = &pb.Packet_Ack{Ack: &pb.Ack{}} }, 0},
		{"response", func(p *pb.Packet) { p.Body = &pb.Packet_Response{Response: &pb.Response{Status: pb.Status_STATUS_OK}} }, 0},
		{"response bad version", func(p *pb.Packet) {
			p.Version = 2
			p.Body = &pb.Packet_Response{Response: &pb.Response{Status: pb.Status_STATUS_OK}}
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &pb.Packet{Version: 1, ClientId: 11, RequestId: 22, Sequence: 33, Op: pb.Operation_OPERATION_SET,
				Body: &pb.Packet_Request{Request: &pb.Request{Key: []byte("k"), Value: []byte("v")}}}
			tc.change(p)
			data, err := proto.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			reply, err := h.Handle(data)
			if err != nil {
				t.Fatal(err)
			}
			if tc.status == 0 {
				if reply != nil {
					t.Fatal("packet must be silently discarded")
				}
				return
			}
			resp, err := protocol.DecodeResponse(reply)
			if err != nil {
				t.Fatal(err)
			}
			if resp.Status != tc.status || resp.Header.ClientID != p.ClientId || resp.Header.RequestID != p.RequestId ||
				resp.Header.Sequence != p.Sequence || resp.Header.Op != protocol.OpCode(p.Op) || resp.Message == "" {
				t.Fatalf("incorrect error response: %#v", resp)
			}
		})
	}
	for _, data := range [][]byte{nil, {255}, bytes.Repeat([]byte{0}, protocol.MaxPacketSize+1)} {
		reply, err := h.Handle(data)
		if err != nil || reply != nil {
			t.Fatalf("invalid packet produced response: %x, %v", reply, err)
		}
	}
	if _, err := store.Get("k"); err == nil {
		t.Fatal("invalid request executed a command")
	}
	// A malformed packet must not break subsequent valid requests.
	data, err := protocol.EncodeRequest(protocol.Request{Header: protocol.Header{Op: protocol.OpPing, ClientID: 1, RequestID: 2, Sequence: 3}})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := h.Handle(data)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := protocol.DecodeResponse(reply)
	if err != nil || resp.Status != protocol.StatusOK {
		t.Fatalf("valid request failed: %#v, %v", resp, err)
	}
}
