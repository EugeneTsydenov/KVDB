package protocol

import (
	"kvdb/internal/protocol/pb"

	"google.golang.org/protobuf/proto"
)

// Header groups packet metadata in Go; it has no separate wire representation.
type Header struct {
	Op        OpCode
	ClientID  uint64
	RequestID uint64
	Sequence  uint32
}

// DecodeHeader recovers reply metadata from a syntactically valid request.
// Version, operation and command arguments may be invalid, but identifiers and
// the request body must be present. Never reply to responses or ACKs.
func DecodeHeader(data []byte) (Header, error) {
	if len(data) == 0 {
		return Header{}, ErrInvalidPacket
	}
	if len(data) > MaxPacketSize {
		return Header{}, ErrPacketTooLarge
	}
	packet := new(pb.Packet)
	if err := proto.Unmarshal(data, packet); err != nil {
		return Header{}, err
	}
	if packet.GetRequest() == nil || packet.GetClientId() == 0 ||
		packet.GetRequestId() == 0 || packet.GetSequence() == 0 {
		return Header{}, ErrInvalidPacket
	}

	return Header{
		Op: OpCode(packet.GetOp()), ClientID: packet.GetClientId(),
		RequestID: packet.GetRequestId(), Sequence: packet.GetSequence(),
	}, nil
}
