package protocol

import (
	"bytes"

	"kvdb/internal/protocol/pb"
)

type Request struct {
	Header Header

	Key     []byte
	Value   []byte
	TTLMs   uint64
	Pattern string
	Cursor  uint64
	Limit   uint32
}

func EncodeRequest(req Request) ([]byte, error) {
	if !req.Header.Op.Valid() {
		return nil, ErrUnknownOpcode
	}

	packet := &pb.Packet{
		Version:   ProtocolVersion,
		ClientId:  req.Header.ClientID,
		RequestId: req.Header.RequestID,
		Sequence:  req.Header.Sequence,
		Op:        req.Header.Op.ToPbOperation(),

		Body: &pb.Packet_Request{
			Request: &pb.Request{
				Key:     req.Key,
				Value:   req.Value,
				TtlMs:   req.TTLMs,
				Pattern: req.Pattern,
				Cursor:  req.Cursor,
				Limit:   req.Limit,
			},
		},
	}

	return Encode(packet)
}

func DecodeRequest(data []byte) (Request, error) {
	packet, err := Decode(data)
	if err != nil {
		return Request{}, err
	}

	body, ok := packet.GetBody().(*pb.Packet_Request)
	if !ok {
		return Request{}, ErrInvalidPacket
	}

	op, err := OpCodeFromPb(packet.GetOp())
	if err != nil {
		return Request{}, err
	}

	req := body.Request

	return Request{
		Header: Header{
			Op:        op,
			ClientID:  packet.GetClientId(),
			RequestID: packet.GetRequestId(),
			Sequence:  packet.GetSequence(),
		},

		Key:     bytes.Clone(req.GetKey()),
		Value:   bytes.Clone(req.GetValue()),
		TTLMs:   req.GetTtlMs(),
		Pattern: req.GetPattern(),
		Cursor:  req.GetCursor(),
		Limit:   req.GetLimit(),
	}, nil
}
