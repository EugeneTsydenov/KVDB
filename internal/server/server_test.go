package server

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

type handlerFunc func([]byte) ([]byte, error)

func (f handlerFunc) Handle(packet []byte) ([]byte, error) { return f(packet) }

func listenUDP(t *testing.T) (*net.UDPConn, string) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, conn.LocalAddr().String()
}

func newRunningServer(t *testing.T, h Handler) (*Server, string, context.CancelFunc, <-chan error) {
	t.Helper()
	probe, addr := listenUDP(t)
	_ = probe.Close()
	s, err := New(addr, h)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { defer close(done); done <- s.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = s.Close()
		select {
		case err, ok := <-done:
			if ok && err != nil {
				t.Errorf("Serve() returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("Serve() did not stop")
		}
	})
	return s, addr, cancel, done
}

func TestNewValidatesHandlerAddressAndReleasesPort(t *testing.T) {
	if _, err := New("127.0.0.1:0", nil); err == nil {
		t.Fatal("New accepted a nil handler")
	}
	zeroPortServer, err := New("127.0.0.1:0", handlerFunc(func([]byte) ([]byte, error) { return nil, nil }))
	if err != nil {
		t.Fatalf("New(127.0.0.1:0): %v", err)
	}
	if err := zeroPortServer.Close(); err != nil {
		t.Fatalf("Close zero-port server: %v", err)
	}
	if _, err := New("not a UDP address", handlerFunc(func([]byte) ([]byte, error) { return nil, nil })); err == nil {
		t.Fatal("New accepted an invalid address")
	}

	occupied, addr := listenUDP(t)
	if _, err := New(addr, handlerFunc(func([]byte) ([]byte, error) { return nil, nil })); err == nil {
		t.Fatal("New bound an already occupied address")
	}
	if err := occupied.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := New(addr, handlerFunc(func([]byte) ([]byte, error) { return nil, nil }))
	if err != nil {
		t.Fatalf("New after releasing port: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close should be harmless, got %v", err)
	}

	rebound, err := net.ListenUDP("udp", mustResolveUDP(t, addr))
	if err != nil {
		t.Fatalf("port was not released by Close: %v", err)
	}
	_ = rebound.Close()
}

func mustResolveUDP(t *testing.T, addr string) *net.UDPAddr {
	t.Helper()
	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestServePassesOnlyDatagramBytesAndRepliesToSender(t *testing.T) {
	got := make(chan []byte, 1)
	wantRequest := append(bytes.Repeat([]byte{0x2a}, 19), 0x7f)
	wantResponse := []byte("response-payload")
	_, addr, _, _ := newRunningServer(t, handlerFunc(func(packet []byte) ([]byte, error) {
		got <- bytes.Clone(packet)
		return bytes.Clone(wantResponse), nil
	}))

	client, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(wantRequest); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !bytes.Equal(buf[:n], wantResponse) {
		t.Fatalf("response=%q want %q", buf[:n], wantResponse)
	}
	select {
	case packet := <-got:
		if !bytes.Equal(packet, wantRequest) {
			t.Fatalf("handler received %d bytes %x, want %d bytes %x", len(packet), packet, len(wantRequest), wantRequest)
		}
	case <-time.After(time.Second):
		t.Fatal("handler was not called")
	}
}

func TestServeSkipsNilAndErrorHandlerResponsesButContinues(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	_, addr, _, _ := newRunningServer(t, handlerFunc(func([]byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		switch calls {
		case 1:
			return nil, errors.New("expected handler error")
		case 2:
			return nil, nil
		default:
			return []byte("third response"), nil
		}
	}))

	client, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(time.Second))
	for i := 0; i < 3; i++ {
		if _, err := client.Write([]byte("0123456789abcdef")); err != nil {
			t.Fatal(err)
		}
	}
	buf := make([]byte, 128)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("server did not process the third request: %v", err)
	}
	if got, want := string(buf[:n]), "third response"; got != want {
		t.Fatalf("response=%q want %q", got, want)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Fatalf("handler calls=%d want 3", calls)
	}
}

func TestServeStopsOnContextCancellationAndClose(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		_, _, cancel, done := newRunningServer(t, handlerFunc(func([]byte) ([]byte, error) { return nil, nil }))
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Serve returned %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Serve remained blocked after context cancellation")
		}
	})

	t.Run("explicit close", func(t *testing.T) {
		s, _, _, done := newRunningServer(t, handlerFunc(func([]byte) ([]byte, error) { return nil, nil }))
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("second Close: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Serve returned %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Serve remained blocked after Close")
		}
	})
}
