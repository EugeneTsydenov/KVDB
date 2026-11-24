package protocol

import (
	"errors"

	"kvdb/internal/protocol/pb"

	"google.golang.org/protobuf/proto"
)

const MaxPacketSize = 1400

var (
	ErrPacketTooLarge = errors.New("packet too large")
	ErrInvalidPacket  = errors.New("invalid packet")
)

func Encode(packet *pb.Packet) ([]byte, error) {
	if err := Validate(packet); err != nil {
		return nil, err
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		return nil, err
	}

	if len(data) > MaxPacketSize {
		return nil, ErrPacketTooLarge
	}

	return data, nil
}

func Decode(data []byte) (*pb.Packet, error) {
	if len(data) == 0 {
		return nil, ErrInvalidPacket
	}

	if len(data) > MaxPacketSize {
		return nil, ErrPacketTooLarge
	}

	packet := new(pb.Packet)
	if err := proto.Unmarshal(data, packet); err != nil {
		return nil, err
	}

	if err := Validate(packet); err != nil {
		return nil, err
	}

	return packet, nil
}
