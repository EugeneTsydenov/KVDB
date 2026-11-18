package integration

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"kvdb/internal/handler"
	"kvdb/internal/protocol"
	"kvdb/internal/server"
	"kvdb/internal/storage"
)

func TestUDPServerSurvivesMalformedDatagrams(t *testing.T) {
	probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.LocalAddr().String()
	_ = probe.Close()

	h, err := handler.New(storage.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	s, err := server.New(addr, h)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = s.Close()
		select {
		case serveErr := <-done:
			if serveErr != nil {
				t.Errorf("Serve: %v", serveErr)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})

	peer, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	validPing := func(id uint64) []byte {
		packet, encodeErr := protocol.EncodeRequest(protocol.Request{Op: protocol.OpPing, RequestID: id})
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		return packet
	}
	base := validPing(1)
	badMagic := clonePacket(base)
	badMagic[0] ^= 0xff
	badVersion := clonePacket(base)
	badVersion[2]++
	badFlags := clonePacket(base)
	badFlags[5] = 1
	unknownOp := clonePacket(base)
	unknownOp[3] = 0xff
	badLength := clonePacket(base)
	binary.BigEndian.PutUint16(badLength[6:8], 1)
	trailing := append(clonePacket(base), 0x01)
	binary.BigEndian.PutUint16(trailing[6:8], 1)

	truncatedField := clonePacket(base[:protocol.HeaderSize])
	truncatedField[3] = byte(protocol.OpGet)
	truncatedField = append(truncatedField, 0x00)
	binary.BigEndian.PutUint16(truncatedField[6:8], 1)

	invalidRegexp, err := protocol.EncodeRequest(protocol.Request{Op: protocol.OpKeys, RequestID: 2, Pattern: "x"})
	if err != nil {
		t.Fatal(err)
	}
	invalidRegexp[18] = '['

	oversizedKey, err := protocol.EncodeRequest(protocol.Request{Op: protocol.OpGet, RequestID: 3, Key: []byte("k")})
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(oversizedKey[16:18], protocol.MaxKeyLen+1)

	longPattern, err := protocol.EncodeRequest(protocol.Request{Op: protocol.OpKeys, RequestID: 4, Pattern: "a"})
	if err != nil {
		t.Fatal(err)
	}
	longPattern = append(clonePacket(longPattern[:18]), bytes.Repeat([]byte{'a'}, protocol.MaxPatternLen)...)
	longPattern = append(longPattern, make([]byte, 10)...)
	binary.BigEndian.PutUint16(longPattern[6:8], uint16(len(longPattern)-protocol.HeaderSize))
	binary.BigEndian.PutUint16(longPattern[16:18], protocol.MaxPatternLen+1)

	cases := []struct {
		name       string
		packet     []byte
		wantReply  bool
		wantStatus protocol.Status
		wantID     uint64
		wantOp     protocol.OpCode
	}{
		{name: "shorter than header", packet: make([]byte, protocol.HeaderSize-1)},
		{name: "over 1400 bytes", packet: make([]byte, protocol.MaxPacketSize+100)},
		{name: "bad magic", packet: badMagic},
		{name: "unsupported version", packet: badVersion}, // #52 limitation: no reply is expected.
		{name: "bad flags", packet: badFlags, wantReply: true, wantStatus: protocol.StatusBadRequest, wantID: 1, wantOp: protocol.OpPing},
		{name: "unknown opcode", packet: unknownOp, wantReply: true, wantStatus: protocol.StatusUnknownOpCode, wantID: 1, wantOp: protocol.OpCode(0xff)},
		{name: "header payload length mismatch", packet: badLength},
		{name: "trailing payload bytes", packet: trailing, wantReply: true, wantStatus: protocol.StatusBadRequest, wantID: 1, wantOp: protocol.OpPing},
		{name: "truncated payload field", packet: truncatedField, wantReply: true, wantStatus: protocol.StatusBadRequest, wantID: 1, wantOp: protocol.OpGet},
		{name: "invalid regexp", packet: invalidRegexp, wantReply: true, wantStatus: protocol.StatusInvalidArgument, wantID: 2, wantOp: protocol.OpKeys},
		{name: "field above limit", packet: oversizedKey, wantReply: true, wantStatus: protocol.StatusTooLarge, wantID: 3, wantOp: protocol.OpGet},
		{name: "pattern above limit", packet: longPattern, wantReply: true, wantStatus: protocol.StatusTooLarge, wantID: 4, wantOp: protocol.OpKeys},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := peer.Write(tc.packet); err != nil {
				t.Fatalf("send malformed packet: %v", err)
			}
			_ = peer.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
			buf := make([]byte, protocol.MaxPacketSize+1)
			n, readErr := peer.Read(buf)
			if !tc.wantReply {
				if readErr == nil {
					t.Fatalf("unexpected response to malformed datagram: %x", buf[:n])
				}
				netErr, ok := readErr.(net.Error)
				if !ok || !netErr.Timeout() {
					t.Fatalf("expected read timeout, got %v", readErr)
				}
			} else {
				if readErr != nil {
					t.Fatalf("expected an error response, got %v", readErr)
				}
				resp, decodeErr := protocol.DecodeResponse(buf[:n])
				if decodeErr != nil {
					t.Fatalf("DecodeResponse: %v", decodeErr)
				}
				if resp.Status != tc.wantStatus {
					t.Fatalf("status=%s want %s (message=%q)", resp.Status, tc.wantStatus, resp.Message)
				}
				if resp.RequestID != tc.wantID || resp.Op != tc.wantOp {
					t.Fatalf("error response header={op:%s id:%d}, want {op:%s id:%d}", resp.Op, resp.RequestID, tc.wantOp, tc.wantID)
				}
			}

			// A known-good packet after every malformed datagram proves the server stays alive.
			id := uint64(100 + i)
			ping, encodeErr := protocol.EncodeRequest(protocol.Request{Op: protocol.OpPing, RequestID: id})
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			if _, err := peer.Write(ping); err != nil {
				t.Fatal(err)
			}
			_ = peer.SetReadDeadline(time.Now().Add(time.Second))
			for {
				n, readErr = peer.Read(buf)
				if readErr != nil {
					t.Fatalf("valid PING after malformed datagram failed: %v", readErr)
				}
				resp, decodeErr := protocol.DecodeResponse(buf[:n])
				if decodeErr != nil {
					continue
				}
				if resp.RequestID != id {
					continue
				}
				if resp.Op != protocol.OpPing || resp.Status != protocol.StatusOK {
					t.Fatalf("unexpected recovery response: %+v", resp)
				}
				break
			}
		})
	}
}

func clonePacket(packet []byte) []byte { return append([]byte(nil), packet...) }
