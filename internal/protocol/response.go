package protocol

import (
	"bytes"
	"errors"
	"slices"

	"kvdb/internal/protocol/pb"
)

type Response struct {
	Header Header

	Status  Status
	Value   []byte
	TTLMs   int64
	Deleted uint64
	Keys    []string
	Message string
}

func EncodeResponse(resp Response) ([]byte, error) {
	if !resp.Status.Valid() {
		return nil, ErrInvalidPacket
	}

	data, err := encodeResponse(resp)
	if err == nil {
		return data, nil
	}

	if !errors.Is(err, ErrPacketTooLarge) {
		return nil, err
	}

	// Replace an oversized response with a small error.
	resp = Response{
		Header:  resp.Header,
		Status:  StatusTooLarge,
		Message: "response exceeds size limit",
	}

	return encodeResponse(resp)
}

func encodeResponse(resp Response) ([]byte, error) {
	// Sort a copy so callers retain ownership of their result slice.
	sortedKeys := slices.Clone(resp.Keys)
	slices.Sort(sortedKeys)
	keys := make([][]byte, len(resp.Keys))

	for i, key := range sortedKeys {
		keys[i] = []byte(key)
	}

	packet := &pb.Packet{
		Version:   ProtocolVersion,
		ClientId:  resp.Header.ClientID,
		RequestId: resp.Header.RequestID,
		Sequence:  resp.Header.Sequence,
		Op:        resp.Header.Op.ToPbOperation(),

		Body: &pb.Packet_Response{
			Response: &pb.Response{
				Status:  resp.Status.ToPbStatus(),
				Value:   resp.Value,
				TtlMs:   resp.TTLMs,
				Deleted: resp.Deleted,
				Keys:    keys,
				Message: resp.Message,
			},
		},
	}

	return Encode(packet)
}

func DecodeResponse(data []byte) (Response, error) {
	packet, err := Decode(data)
	if err != nil {
		return Response{}, err
	}

	body, ok := packet.GetBody().(*pb.Packet_Response)
	if !ok {
		return Response{}, ErrInvalidPacket
	}

	// Error responses preserve unknown operation numbers from the request.
	op := OpCode(packet.GetOp())

	status, err := StatusFromPb(body.Response.GetStatus())
	if err != nil {
		return Response{}, err
	}

	pbResp := body.Response

	keys := make([]string, len(pbResp.GetKeys()))

	for i, key := range pbResp.GetKeys() {
		keys[i] = string(key)
	}

	return Response{
		Header: Header{
			Op:        op,
			ClientID:  packet.GetClientId(),
			RequestID: packet.GetRequestId(),
			Sequence:  packet.GetSequence(),
		},

		Status:  status,
		Value:   bytes.Clone(pbResp.GetValue()),
		TTLMs:   pbResp.GetTtlMs(),
		Deleted: pbResp.GetDeleted(),
		Keys:    keys,
		Message: pbResp.GetMessage(),
	}, nil
}
