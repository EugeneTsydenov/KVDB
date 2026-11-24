package protocol

import "kvdb/internal/protocol/pb"

// EncodeAck confirms acceptance using the original request metadata.
func EncodeAck(header Header) ([]byte, error) {
	return Encode(&pb.Packet{
		Version: ProtocolVersion, ClientId: header.ClientID,
		RequestId: header.RequestID, Sequence: header.Sequence,
		Op: header.Op.ToPbOperation(), Body: &pb.Packet_Ack{Ack: &pb.Ack{}},
	})
}

// DecodeAck validates an ACK and returns its request metadata.
func DecodeAck(data []byte) (Header, error) {
	packet, err := Decode(data)
	if err != nil {
		return Header{}, err
	}
	if packet.GetAck() == nil {
		return Header{}, ErrInvalidPacket
	}

	return Header{
		Op: OpCode(packet.GetOp()), ClientID: packet.GetClientId(),
		RequestID: packet.GetRequestId(), Sequence: packet.GetSequence(),
	}, nil
}
