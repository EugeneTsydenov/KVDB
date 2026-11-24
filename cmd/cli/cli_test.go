package main

import (
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"kvdb/internal/client"
	"kvdb/internal/protocol"
)

func captureFile(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	old := *target
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	*target = writer
	fn()
	_ = writer.Close()
	*target = old
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func captureStdout(t *testing.T, fn func()) string { return captureFile(t, &os.Stdout, fn) }
func captureStderr(t *testing.T, fn func()) string { return captureFile(t, &os.Stderr, fn) }

func TestParseRequestSupportsAllCommandsAndCaseInsensitiveNames(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want protocol.Request
	}{
		{"PING", []string{"PiNg"}, protocol.Request{Op: protocol.OpPing}},
		{"GET", []string{"gEt", "key"}, protocol.Request{Op: protocol.OpGet, Key: []byte("key")}},
		{"SET", []string{"SET", "key", "value", "1500"}, protocol.Request{Op: protocol.OpSet, Key: []byte("key"), Value: []byte("value"), TTLMs: 1500}},
		{"SET without TTL", []string{"set", "key", "value"}, protocol.Request{Op: protocol.OpSet, Key: []byte("key"), Value: []byte("value")}},
		{"SETNX", []string{"SeTnX", "key", "value", "20"}, protocol.Request{Op: protocol.OpSetNX, Key: []byte("key"), Value: []byte("value"), TTLMs: 20}},
		{"DEL", []string{"del", "key"}, protocol.Request{Op: protocol.OpDel, Key: []byte("key")}},
		{"TTL", []string{"ttl", "key"}, protocol.Request{Op: protocol.OpTTL, Key: []byte("key")}},
		{"EXPIRE", []string{"expire", "key", "45"}, protocol.Request{Op: protocol.OpExpire, Key: []byte("key"), TTLMs: 45}},
		{"KEYS", []string{"keys", "^user:"}, protocol.Request{Op: protocol.OpKeys, Pattern: "^user:"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRequest(tc.args)
			if err != nil {
				t.Fatalf("parseRequest: %v", err)
			}
			if got.Op != tc.want.Op || string(got.Key) != string(tc.want.Key) || string(got.Value) != string(tc.want.Value) || got.TTLMs != tc.want.TTLMs || got.Pattern != tc.want.Pattern {
				t.Fatalf("request=%+v want %+v", got, tc.want)
			}
		})
	}
}

func TestParseRequestRejectsInvalidCommandsAndArguments(t *testing.T) {
	cases := [][]string{
		{}, {"unknown"}, {"ping", "extra"}, {"get"}, {"get", "key", "extra"},
		{"set", "key"}, {"set", "key", "value", "1", "extra"}, {"setnx", "key"},
		{"del"}, {"del", "key", "extra"}, {"ttl"}, {"expire", "key"}, {"expire", "key", "1", "extra"}, {"keys"}, {"keys", ".*", "extra"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseRequest(args); err == nil {
				t.Fatalf("parseRequest(%q) succeeded", args)
			}
		})
	}
}

func TestParseTTLValidation(t *testing.T) {
	valid, err := parseRequest([]string{"set", "k", "v", "0"})
	if err != nil || valid.TTLMs != 0 {
		t.Fatalf("zero SET TTL: request=%+v err=%v", valid, err)
	}
	valid, err = parseRequest([]string{"expire", "k", "9223372036854"})
	if err != nil || valid.TTLMs != uint64(protocol.MaxTTLMs) {
		t.Fatalf("maximum EXPIRE TTL: request=%+v err=%v", valid, err)
	}

	cases := [][]string{
		{"set", "k", "v", "-1"},
		{"set", "k", "v", "abc"},
		{"set", "k", "v", "9223372036855"},
		{"setnx", "k", "v", "18446744073709551616"},
		{"expire", "k", "0"},
		{"expire", "k", "-10"},
		{"expire", "k", "not-a-number"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseRequest(args); err == nil {
				t.Fatalf("parseRequest(%q) succeeded", args)
			}
		})
	}
}

func TestPrintResponseOutputsStatusesAndValues(t *testing.T) {
	cases := []struct {
		name    string
		resp    protocol.Response
		want    string
		wantErr string
	}{
		{"PING", protocol.Response{Op: protocol.OpPing, Status: protocol.StatusOK}, "PONG\n", ""},
		{"SET", protocol.Response{Op: protocol.OpSet, Status: protocol.StatusOK}, "OK\n", ""},
		{"SETNX", protocol.Response{Op: protocol.OpSetNX, Status: protocol.StatusOK}, "OK\n", ""},
		{"EXPIRE", protocol.Response{Op: protocol.OpExpire, Status: protocol.StatusOK}, "OK\n", ""},
		{"GET", protocol.Response{Op: protocol.OpGet, Status: protocol.StatusOK, Value: []byte("hello")}, "hello\n", ""},
		{"GET binary", protocol.Response{Op: protocol.OpGet, Status: protocol.StatusOK, Value: []byte{0xff, 0x00}}, "\"\\xff\\x00\"\n", ""},
		{"DEL", protocol.Response{Op: protocol.OpDel, Status: protocol.StatusOK, Deleted: 1}, "(integer) 1\n", ""},
		{"TTL", protocol.Response{Op: protocol.OpTTL, Status: protocol.StatusOK, TTLMs: -1}, "(integer) -1\n", ""},
		{"KEYS", protocol.Response{Op: protocol.OpKeys, Status: protocol.StatusOK, Keys: []string{"alpha", "beta"}}, "1) \"alpha\"\n2) \"beta\"\n", ""},
		{"empty KEYS", protocol.Response{Op: protocol.OpKeys, Status: protocol.StatusOK}, "(empty list)\n", ""},
		{"NOT_FOUND", protocol.Response{Op: protocol.OpGet, Status: protocol.StatusNotFound}, "NOT_FOUND\n", ""},
		{"NOT_SET", protocol.Response{Op: protocol.OpSetNX, Status: protocol.StatusNotSet}, "NOT_SET\n", ""},
		{"server error", protocol.Response{Op: protocol.OpSet, Status: protocol.StatusInvalidArgument, Message: "bad argument"}, "", "INVALID_ARGUMENT: bad argument"},
		{"server error without message", protocol.Response{Op: protocol.OpSet, Status: protocol.StatusInternalError}, "", "server returned INTERNAL_ERROR"},
		{"unknown response opcode", protocol.Response{Op: 0xff, Status: protocol.StatusOK}, "", "unexpected opcode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var callErr error
			out := captureStdout(t, func() { callErr = printResponse(tc.resp) })
			if out != tc.want {
				t.Fatalf("stdout=%q want %q", out, tc.want)
			}
			if tc.wantErr == "" && callErr != nil {
				t.Fatalf("unexpected error: %v", callErr)
			}
			if tc.wantErr != "" && (callErr == nil || !strings.Contains(callErr.Error(), tc.wantErr)) {
				t.Fatalf("error=%v want substring %q", callErr, tc.wantErr)
			}
		})
	}
}

func TestPrintValueKeepsUTF8AndEscapesBinary(t *testing.T) {
	out := captureStdout(t, func() { printValue([]byte("Привет, мир")) })
	if out != "Привет, мир\n" {
		t.Fatalf("UTF-8 output=%q", out)
	}
	out = captureStdout(t, func() { printValue([]byte{0xff, 0x00}) })
	if out != "\"\\xff\\x00\"\n" {
		t.Fatalf("binary output=%q", out)
	}
	out = captureStdout(t, func() { printValue(nil) })
	if out != "\n" {
		t.Fatalf("empty output=%q", out)
	}
}

func TestRunParsesFlagsAndExecutesPing(t *testing.T) {
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peerErr := make(chan error, 1)
	go func() {
		buf := make([]byte, protocol.MaxPacketSize+1)
		_ = peer.SetReadDeadline(time.Now().Add(time.Second))
		n, addr, readErr := peer.ReadFromUDP(buf)
		if readErr != nil {
			peerErr <- readErr
			return
		}
		req, decodeErr := protocol.DecodeRequest(buf[:n])
		if decodeErr != nil {
			peerErr <- decodeErr
			return
		}
		if req.Op != protocol.OpPing {
			peerErr <- errorsForUnexpectedOp(req.Op)
			return
		}
		packet, encodeErr := protocol.EncodeResponse(protocol.Response{Op: req.Op, Status: protocol.StatusOK, RequestID: req.RequestID})
		if encodeErr == nil {
			_, encodeErr = peer.WriteToUDP(packet, addr)
		}
		peerErr <- encodeErr
	}()

	out := captureStdout(t, func() {
		err = run([]string{"-addr", peer.LocalAddr().String(), "-timeout", "500ms", "ping"})
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "PONG\n" {
		t.Fatalf("stdout=%q", out)
	}
	if err := <-peerErr; err != nil {
		t.Fatal(err)
	}
}

func errorsForUnexpectedOp(op protocol.OpCode) error { return &unexpectedOpError{op: op} }

type unexpectedOpError struct{ op protocol.OpCode }

func (e *unexpectedOpError) Error() string { return "unexpected CLI request opcode: " + e.op.String() }

func TestRunReportsUsageAndInvalidOptions(t *testing.T) {
	var runErr error
	stderr := captureStderr(t, func() { runErr = run(nil) })
	if runErr == nil || !strings.Contains(runErr.Error(), "command is required") {
		t.Fatalf("run error=%v", runErr)
	}
	if !strings.Contains(stderr, "Usage:") || !strings.Contains(stderr, "Commands:") {
		t.Fatalf("usage output missing: %q", stderr)
	}

	stderr = captureStderr(t, func() { runErr = run([]string{"-unknown", "ping"}) })
	if runErr == nil || !strings.Contains(stderr, "unknown") {
		t.Fatalf("invalid option: err=%v stderr=%q", runErr, stderr)
	}
	if err := run([]string{"-timeout", "0s", "ping"}); err == nil {
		t.Fatal("run accepted zero timeout")
	}
	if err := run([]string{"-addr", "127.0.0.1:not-a-port", "ping"}); err == nil {
		t.Fatal("run accepted an invalid server address")
	}
}

func TestRunReturnsServerStatusErrors(t *testing.T) {
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	go func() {
		buf := make([]byte, protocol.MaxPacketSize+1)
		n, addr, readErr := peer.ReadFromUDP(buf)
		if readErr != nil {
			return
		}
		req, decodeErr := protocol.DecodeRequest(buf[:n])
		if decodeErr != nil {
			return
		}
		packet, encodeErr := protocol.EncodeResponse(protocol.Response{Op: req.Op, Status: protocol.StatusInvalidArgument, RequestID: req.RequestID, Message: "invalid test request"})
		if encodeErr == nil {
			_, _ = peer.WriteToUDP(packet, addr)
		}
	}()
	var runErr error
	_ = captureStdout(t, func() {
		runErr = run([]string{"-addr", peer.LocalAddr().String(), "-timeout", client.DefaultTimeout.String(), "ping"})
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "INVALID_ARGUMENT") {
		t.Fatalf("run error=%v", runErr)
	}
}
