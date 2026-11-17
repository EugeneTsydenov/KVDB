package server

import (
	"context"
	"log"
	"net"

	"kvdb/internal/protocol"
)

type Handler interface {
	Handle(packet []byte) ([]byte, error)
}

type Server struct {
	conn *net.UDPConn
	h    Handler
}

func New(addr string, h Handler) (*Server, error) {
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
	buf := make([]byte, protocol.MaxPacketSize)

	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read error: %w", err)
			continue
		}

		if n > protocol.MaxPacketSize || n < protocol.HeaderSize {
			log.Printf("invalid packet len: %d", n)
		}

		s.h.Handle(buf[:n])
	}

	return nil
}

func (s *Server) Close() error {
	return nil
}
