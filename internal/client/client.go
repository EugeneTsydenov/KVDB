package client

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"kvdb/internal/protocol"
)

const DefaultTimeout = 2 * time.Second

// Client communicates with KVDB over UDP.
type Client struct {
	conn    *net.UDPConn
	timeout time.Duration
	mu      sync.Mutex
	nextID  atomic.Uint64
}

// New creates a UDP client connected to the given server address.
func New(addr string, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		return nil, errors.New("timeout must be positive")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve UDP address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial UDP: %w", err)
	}

	c := &Client{
		conn:    conn,
		timeout: timeout,
	}
	c.nextID.Store(uint64(time.Now().UnixNano()))

	return c, nil
}

// Do sends one request and waits for its response.
// Requests sharing one Client are executed sequentially.
func (c *Client) Do(req protocol.Request) (protocol.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req.RequestID = c.nextID.Add(1)

	packet, err := protocol.EncodeRequest(req)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("encode request: %w", err)
	}

	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return protocol.Response{}, fmt.Errorf("set deadline: %w", err)
	}

	n, err := c.conn.Write(packet)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("send request: %w", err)
	}
	if n != len(packet) {
		return protocol.Response{}, fmt.Errorf(
			"short UDP write: %d of %d bytes", n, len(packet),
		)
	}

	buf := make([]byte, protocol.MaxPacketSize+1)

	n, err = c.conn.Read(buf)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return protocol.Response{}, fmt.Errorf("client closed: %w", err)
		}

		var netErr net.Error
		if ok := errors.As(err, &netErr); ok && netErr.Timeout() {
			return protocol.Response{}, fmt.Errorf(
				"server response timeout: %w", err,
			)
		}

		return protocol.Response{}, fmt.Errorf("read response: %w", err)
	}

	if n > protocol.MaxPacketSize {
		return protocol.Response{}, protocol.ErrTooLarge
	}

	resp, err := protocol.DecodeResponse(buf[:n])
	if err != nil {
		return protocol.Response{}, fmt.Errorf("decode response: %w", err)
	}

	if resp.RequestID != req.RequestID {
		return protocol.Response{}, fmt.Errorf(
			"request ID mismatch: expected %d, got %d",
			req.RequestID,
			resp.RequestID,
		)
	}

	if resp.Op != req.Op {
		return protocol.Response{}, fmt.Errorf(
			"opcode mismatch: expected %s, got %s",
			req.Op,
			resp.Op,
		)
	}

	return resp, nil
}

// Close releases the client's UDP socket.
func (c *Client) Close() error {
	return c.conn.Close()
}
