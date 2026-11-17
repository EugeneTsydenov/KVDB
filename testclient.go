package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"kvdb/internal/protocol"
)

func main() {
	conn, err := net.Dial("udp", "127.0.0.1:9000")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	req := protocol.Request{
		Op:        protocol.OpGet,
		RequestID: 1,
		Key:       []byte("Evgeny"),
	}

	packet, err := protocol.EncodeRequest(req)
	if err != nil {
		log.Fatal(err)
	}

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Fatal(err)
	}

	if _, err := conn.Write(packet); err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, protocol.MaxPacketSize)

	n, err := conn.Read(buf)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := protocol.DecodeResponse(buf[:n])
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Status:", resp.Status)
	fmt.Println("Value:", string(resp.Value))
}
