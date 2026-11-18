package integration

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"kvdb/internal/client"
	"kvdb/internal/handler"
	"kvdb/internal/protocol"
	"kvdb/internal/server"
	"kvdb/internal/storage"
)

func startKVDB(t *testing.T, timeout time.Duration) *client.Client {
	t.Helper()
	probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.LocalAddr().String()
	_ = probe.Close()

	store := storage.NewMemoryStore()
	h, err := handler.New(store)
	if err != nil {
		t.Fatal(err)
	}
	s, err := server.New(addr, h)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = s.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})

	c, err := client.New(addr, timeout)
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestUDPFullCommandFlow(t *testing.T) {
	c := startKVDB(t, time.Second)
	seenIDs := make(map[uint64]struct{})
	call := func(req protocol.Request, wantOp protocol.OpCode) protocol.Response {
		t.Helper()
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("Do(%s): %v", req.Op, err)
		}
		if resp.Op != wantOp {
			t.Fatalf("response opcode=%s want %s", resp.Op, wantOp)
		}
		if resp.RequestID == 0 {
			t.Fatal("server response has zero RequestID")
		}
		if _, exists := seenIDs[resp.RequestID]; exists {
			t.Fatalf("reused RequestID %d", resp.RequestID)
		}
		seenIDs[resp.RequestID] = struct{}{}
		return resp
	}
	assertStatus := func(resp protocol.Response, status protocol.Status) {
		t.Helper()
		if resp.Status != status {
			t.Fatalf("status=%s want %s (message=%q)", resp.Status, status, resp.Message)
		}
	}

	ping := call(protocol.Request{Op: protocol.OpPing}, protocol.OpPing)
	assertStatus(ping, protocol.StatusOK)

	set := call(protocol.Request{Op: protocol.OpSet, Key: []byte("alpha"), Value: []byte("value")}, protocol.OpSet)
	assertStatus(set, protocol.StatusOK)
	get := call(protocol.Request{Op: protocol.OpGet, Key: []byte("alpha")}, protocol.OpGet)
	assertStatus(get, protocol.StatusOK)
	if string(get.Value) != "value" {
		t.Fatalf("GET value=%q", get.Value)
	}

	setnx := call(protocol.Request{Op: protocol.OpSetNX, Key: []byte("new"), Value: []byte("first")}, protocol.OpSetNX)
	assertStatus(setnx, protocol.StatusOK)
	setnxAgain := call(protocol.Request{Op: protocol.OpSetNX, Key: []byte("new"), Value: []byte("second")}, protocol.OpSetNX)
	assertStatus(setnxAgain, protocol.StatusNotSet)
	getNew := call(protocol.Request{Op: protocol.OpGet, Key: []byte("new")}, protocol.OpGet)
	assertStatus(getNew, protocol.StatusOK)
	if string(getNew.Value) != "first" {
		t.Fatalf("SETNX overwrote value: %q", getNew.Value)
	}

	del := call(protocol.Request{Op: protocol.OpDel, Key: []byte("alpha")}, protocol.OpDel)
	assertStatus(del, protocol.StatusOK)
	if del.Deleted != 1 {
		t.Fatalf("DEL count=%d want 1", del.Deleted)
	}
	missing := call(protocol.Request{Op: protocol.OpGet, Key: []byte("alpha")}, protocol.OpGet)
	assertStatus(missing, protocol.StatusNotFound)

	noTTL := call(protocol.Request{Op: protocol.OpTTL, Key: []byte("new")}, protocol.OpTTL)
	assertStatus(noTTL, protocol.StatusOK)
	if noTTL.TTLMs != -1 {
		t.Fatalf("TTL without expiry=%d want -1", noTTL.TTLMs)
	}
	setTTL := call(protocol.Request{Op: protocol.OpSet, Key: []byte("temporary"), Value: []byte("v"), TTLMs: 2000}, protocol.OpSet)
	assertStatus(setTTL, protocol.StatusOK)
	expire := call(protocol.Request{Op: protocol.OpExpire, Key: []byte("new"), TTLMs: 2500}, protocol.OpExpire)
	assertStatus(expire, protocol.StatusOK)
	ttl := call(protocol.Request{Op: protocol.OpTTL, Key: []byte("new")}, protocol.OpTTL)
	assertStatus(ttl, protocol.StatusOK)
	if ttl.TTLMs <= 0 || ttl.TTLMs > 2500 {
		t.Fatalf("TTL out of range: %d", ttl.TTLMs)
	}

	keys := call(protocol.Request{Op: protocol.OpKeys, Pattern: "^(new|temporary)$"}, protocol.OpKeys)
	assertStatus(keys, protocol.StatusOK)
	if len(keys.Keys) != 2 {
		t.Fatalf("KEYS returned %v", keys.Keys)
	}
	empty := call(protocol.Request{Op: protocol.OpKeys, Pattern: "^missing"}, protocol.OpKeys)
	assertStatus(empty, protocol.StatusOK)
	if len(empty.Keys) != 0 {
		t.Fatalf("empty KEYS result=%v", empty.Keys)
	}
}

func TestUDPEndToEndKeysTooLarge(t *testing.T) {
	c := startKVDB(t, time.Second)
	for i := 0; i < 8; i++ {
		key := fmt.Sprintf("bulk-%02d%s", i, strings.Repeat("x", 230))
		resp, err := c.Do(protocol.Request{Op: protocol.OpSet, Key: []byte(key), Value: []byte("v")})
		if err != nil {
			t.Fatalf("SET large key %d: %v", i, err)
		}
		if resp.Status != protocol.StatusOK {
			t.Fatalf("SET large key status=%s", resp.Status)
		}
	}
	resp, err := c.Do(protocol.Request{Op: protocol.OpKeys, Pattern: "^bulk-"})
	if err != nil {
		t.Fatalf("KEYS large list: %v", err)
	}
	if resp.Op != protocol.OpKeys || resp.Status != protocol.StatusTooLarge {
		t.Fatalf("KEYS response={op:%s status:%s message:%q}, want KEYS/TOO_LARGE", resp.Op, resp.Status, resp.Message)
	}
}
