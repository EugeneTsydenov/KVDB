package server

import (
	"context"
	"net"
)

type Server struct {
	conn *net.UDPConn
}

func New(addr string) (*Server, error) {
	uAddr, err := net.ResolveUDPAddr("UDP", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("UDP", uAddr)
	if err != nil {
		return nil, err
	}

	return &Server{
		conn: conn,
	}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	return nil
}

func (s *Server) Close() error {
	return nil
}
