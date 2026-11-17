package server

import (
	"context"
	"errors"
	"fmt"
	"kvdb/internal/protocol"
	"log"
	"net"
	"sync"
)

type Handler interface {
	Handle(packet []byte) ([]byte, error)
}

type Server struct {
	conn      *net.UDPConn
	h         Handler
	closeOnce sync.Once
	closeErr  error
}

func New(addr string, h Handler) (*Server, error) {
	if h == nil {
		return nil, errors.New("handler must not be nil")
	}

	uAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", uAddr)
	if err != nil {
		return nil, err
	}

	return &Server{
		conn: conn,
		h:    h,
	}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-done:
		}
	}()

	buf := make([]byte, protocol.MaxPacketSize+1)

	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}

			return fmt.Errorf("read udp: %w", err)
		}

		if n > protocol.MaxPacketSize || n < protocol.HeaderSize {
			log.Printf("invalid packet len: %d", n)

			continue
		}

		resp, err := s.h.Handle(buf[:n])
		if err != nil {
			log.Printf("handle error: %v", err)

			continue
		}

		if resp == nil {
			continue
		}

		if _, err := s.conn.WriteToUDP(resp, clientAddr); err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}

			log.Printf("write udp: %v", err)
		}
	}
}

func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		if s.conn != nil {
			s.closeErr = s.conn.Close()
		}
	})

	return s.closeErr
}
