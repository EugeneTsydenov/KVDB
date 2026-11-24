package client

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"kvdb/internal/protocol"
)

func newPeer(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestNewValidatesAddressAndTimeout(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Millisecond} {
		if _, err := New("127.0.0.1:9000", timeout); err == nil {
			t.Fatalf("New accepted timeout %s", timeout)
		}
	}
	if _, err := New("127.0.0.1:not-a-port", time.Second); err == nil {
		t.Fatal("New accepted an invalid UDP address")
	}
	peer := newPeer(t)
	c, err := New(peer.LocalAddr().String(), 100*time.Millisecond)
	if err != nil {
		t.Fatalf("New with valid address: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCloseReleasesClientSocket(t *testing.T) {
	peer := newPeer(t)
	c, err := New(peer.LocalAddr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	local, ok := c.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("LocalAddr has type %T", c.conn.LocalAddr())
	}
	localCopy := *local
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	rebound, err := net.ListenUDP("udp", &localCopy)
	if err != nil {
		t.Fatalf("client socket was not released: %v", err)
	}
	_ = rebound.Close()
}

func TestDoEncodesRequestAndGeneratesUniqueRequestIDs(t *testing.T) {
	peer := newPeer(t)
	c, err := New(peer.LocalAddr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	type peerResult struct {
		req protocol.Request
		err error
	}
	seen := make(chan peerResult, 2)
	go func() {
		for i := 0; i < 2; i++ {
			buf := make([]byte, protocol.MaxPacketSize+1)
			n, addr, readErr := peer.ReadFromUDP(buf)
			if readErr != nil {
				seen <- peerResult{err: readErr}
				return
			}
			req, decodeErr := protocol.DecodeRequest(buf[:n])
			if decodeErr != nil {
				seen <- peerResult{err: decodeErr}
				return
			}
			resp, encodeErr := protocol.EncodeResponse(protocol.Response{
				Op: req.Op, Status: protocol.StatusOK, RequestID: req.RequestID, Value: req.Key,
			})
			if encodeErr == nil {
				_, encodeErr = peer.WriteToUDP(resp, addr)
			}
			seen <- peerResult{req: req, err: encodeErr}
			if encodeErr != nil {
				return
			}
		}
	}()

	first, err := c.Do(protocol.Request{Op: protocol.OpGet, RequestID: 555, Key: []byte("first")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Do(protocol.Request{Op: protocol.OpGet, RequestID: 555, Key: []byte("second")})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Value) != "first" || string(second.Value) != "second" {
		t.Fatalf("unexpected values: %q and %q", first.Value, second.Value)
	}
	if first.RequestID == 0 || second.RequestID == 0 || first.RequestID == second.RequestID || first.RequestID == 555 {
		t.Fatalf("request IDs were not generated uniquely: %d, %d", first.RequestID, second.RequestID)
	}
	for i := 0; i < 2; i++ {
		result := <-seen
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.req.Op != protocol.OpGet || result.req.RequestID == 555 {
			t.Fatalf("peer received unexpected request: %+v", result.req)
		}
		if len(result.req.Key) == 0 {
			t.Fatalf("peer received empty key: %+v", result.req)
		}
	}
}

func TestDoRejectsMalformedOrMismatchedResponses(t *testing.T) {
	cases := []struct {
		name         string
		packet       func(protocol.Request) []byte
		wantTooLarge bool
	}{
		{
			name: "request ID mismatch",
			packet: func(req protocol.Request) []byte {
				p, _ := protocol.EncodeResponse(protocol.Response{Op: req.Op, Status: protocol.StatusOK, RequestID: req.RequestID + 1})
				return p
			},
		},
		{
			name: "opcode mismatch",
			packet: func(req protocol.Request) []byte {
				p, _ := protocol.EncodeResponse(protocol.Response{Op: protocol.OpPing, Status: protocol.StatusOK, RequestID: req.RequestID})
				return p
			},
		},
		{name: "malformed response", packet: func(protocol.Request) []byte { return []byte{0x01, 0x02, 0x03} }},
		{name: "oversized response", packet: func(protocol.Request) []byte { return bytes.Repeat([]byte{0xaa}, protocol.MaxPacketSize+1) }, wantTooLarge: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer := newPeer(t)
			c, err := New(peer.LocalAddr().String(), 300*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			peerDone := make(chan error, 1)
			go func() {
				buf := make([]byte, protocol.MaxPacketSize+1)
				_ = peer.SetReadDeadline(time.Now().Add(time.Second))
				n, addr, readErr := peer.ReadFromUDP(buf)
				if readErr != nil {
					peerDone <- readErr
					return
				}
				req, decodeErr := protocol.DecodeRequest(buf[:n])
				if decodeErr != nil {
					peerDone <- decodeErr
					return
				}
				_, writeErr := peer.WriteToUDP(tc.packet(req), addr)
				peerDone <- writeErr
			}()
			_, err = c.Do(protocol.Request{Op: protocol.OpGet, Key: []byte("k")})
			if err == nil {
				t.Fatal("Do accepted an invalid response")
			}
			if peerErr := <-peerDone; peerErr != nil {
				t.Fatalf("peer failed: %v", peerErr)
			}
			if tc.wantTooLarge && !errors.Is(err, protocol.ErrTooLarge) {
				t.Fatalf("error=%v, want protocol.ErrTooLarge", err)
			}
		})
	}
}

func TestDoTimeoutDoesNotRetry(t *testing.T) {
	peer := newPeer(t)
	c, err := New(peer.LocalAddr().String(), 60*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	count := make(chan int, 1)
	go func() {
		buf := make([]byte, protocol.MaxPacketSize+1)
		got := 0
		_ = peer.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		for {
			if _, _, readErr := peer.ReadFromUDP(buf); readErr != nil {
				break
			}
			got++
		}
		count <- got
	}()
	start := time.Now()
	_, err = c.Do(protocol.Request{Op: protocol.OpPing})
	if err == nil {
		t.Fatal("Do succeeded despite no response")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("Do error=%v, want a network timeout", err)
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("Do returned before configured timeout: %s", time.Since(start))
	}
	select {
	case got := <-count:
		if got != 1 {
			t.Fatalf("client sent %d datagrams, want exactly one", got)
		}
	case <-time.After(time.Second):
		t.Fatal("peer did not finish counting datagrams")
	}
}

func TestConcurrentDoCallsOnOneClientDoNotMixResponses(t *testing.T) {
	const count = 16
	peer := newPeer(t)
	c, err := New(peer.LocalAddr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	serverErr := make(chan error, 1)
	go func() {
		_ = peer.SetReadDeadline(time.Now().Add(3 * time.Second))
		for i := 0; i < count; i++ {
			buf := make([]byte, protocol.MaxPacketSize+1)
			n, addr, readErr := peer.ReadFromUDP(buf)
			if readErr != nil {
				serverErr <- readErr
				return
			}
			req, decodeErr := protocol.DecodeRequest(buf[:n])
			if decodeErr != nil {
				serverErr <- decodeErr
				return
			}
			resp, encodeErr := protocol.EncodeResponse(protocol.Response{
				Op: req.Op, Status: protocol.StatusOK, RequestID: req.RequestID, Value: req.Key,
			})
			if encodeErr != nil {
				serverErr <- encodeErr
				return
			}
			if _, writeErr := peer.WriteToUDP(resp, addr); writeErr != nil {
				serverErr <- writeErr
				return
			}
		}
		serverErr <- nil
	}()

	type result struct {
		key  string
		resp protocol.Response
		err  error
	}
	results := make(chan result, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("key-%02d", i)
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			resp, doErr := c.Do(protocol.Request{Op: protocol.OpGet, Key: []byte(key)})
			results <- result{key: key, resp: resp, err: doErr}
		}(key)
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			t.Errorf("Do(%s): %v", result.key, result.err)
			continue
		}
		if string(result.resp.Value) != result.key {
			t.Errorf("Do(%s) returned value %q", result.key, result.resp.Value)
		}
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}
