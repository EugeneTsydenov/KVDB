package client

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"kvdb/internal/protocol"
)

const DefaultTimeout = 2 * time.Second

// Client communicates with KVDB over UDP.
type Client struct {
	conn     *net.UDPConn
	timeout  time.Duration
	mu       sync.Mutex
	clientID uint64
	nextID   uint64
	sequence uint32
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
	clientID, err := newClientID()
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial UDP: %w", err)
	}

	c := &Client{
		conn:     conn,
		timeout:  timeout,
		clientID: clientID,
	}

	return c, nil
}

// Do sends one request and waits for its response.
// Requests sharing one Client are executed sequentially.
//
//nolint:cyclop,funlen
func (c *Client) Do(req protocol.Request) (protocol.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sequence == ^uint32(0) || c.nextID == ^uint64(0) {
		id, err := newClientID()
		if err != nil {
			return protocol.Response{}, err
		}
		c.clientID, c.nextID, c.sequence = id, 0, 0
	}
	c.nextID++
	c.sequence++
	req.Header.ClientID = c.clientID
	req.Header.RequestID = c.nextID
	req.Header.Sequence = c.sequence

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

	for {
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
			continue
		}
		if _, err := protocol.DecodeAck(buf[:n]); err == nil {
			// ACK never completes a request or extends its deadline.
			continue
		}

		resp, err := protocol.DecodeResponse(buf[:n])
		if err != nil {
			continue
		}

		if resp.Header != req.Header {
			continue
		}

		return resp, nil
	}
}

func newClientID() (uint64, error) {
	for {
		var data [8]byte
		if _, err := rand.Read(data[:]); err != nil {
			return 0, fmt.Errorf("generate client ID: %w", err)
		}
		if id := binary.LittleEndian.Uint64(data[:]); id != 0 {
			return id, nil
		}
	}
}

// Close releases the client's UDP socket.
func (c *Client) Close() error {
	return c.conn.Close()
}
