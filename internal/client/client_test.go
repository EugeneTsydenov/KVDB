package client_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"kvdb/internal/client"
	"kvdb/internal/protocol"
)

func TestWaitForMatchingResponseAfterAck(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- replyWithAckAndResponse(conn) }()
	c, err := client.New(conn.LocalAddr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for i := 1; i <= 2; i++ {
		resp, err := c.Do(protocol.Request{Header: protocol.Header{Op: protocol.OpGet}, Key: []byte("k")})
		if err != nil {
			t.Fatal(err)
		}
		if string(resp.Value) != "final" || resp.Header.ClientID == 0 || resp.Header.Sequence != uint32(i) || resp.Header.RequestID != uint64(i) {
			t.Fatalf("incorrect response: %#v", resp)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func replyWithAckAndResponse(conn *net.UDPConn) error {
	var clientID uint64
	for i := 0; i < 2; i++ {
		data := make([]byte, protocol.MaxPacketSize+1)
		n, addr, err := conn.ReadFromUDP(data)
		if err != nil {
			return err
		}
		req, err := protocol.DecodeRequest(data[:n])
		if err != nil {
			return err
		}
		if i > 0 && req.Header.ClientID != clientID {
			return fmt.Errorf("client session changed")
		}
		clientID = req.Header.ClientID
		ack, err := protocol.EncodeAck(req.Header)
		if err != nil {
			return err
		}
		response := protocol.Response{Header: req.Header, Status: protocol.StatusOK, Value: []byte("final")}
		final, err := protocol.EncodeResponse(response)
		if err != nil {
			return err
		}
		mismatched := response
		mismatched.Header.Sequence++
		wrongSequence, err := protocol.EncodeResponse(mismatched)
		if err != nil {
			return err
		}
		mismatched = response
		mismatched.Header.ClientID++
		wrongClient, err := protocol.EncodeResponse(mismatched)
		if err != nil {
			return err
		}
		mismatched = response
		mismatched.Header.RequestID++
		wrongRequest, err := protocol.EncodeResponse(mismatched)
		if err != nil {
			return err
		}
		for _, msg := range [][]byte{ack, wrongSequence, wrongClient, wrongRequest, {255}, final} {
			if _, err := conn.WriteToUDP(msg, addr); err != nil {
				return err
			}
		}
	}
	return nil
}
