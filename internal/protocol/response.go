package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"math"
)

// Response holds the result of a command.
// Op and Status tell the code which fields to use.
type Response struct {
	Op        OpCode
	Status    Status
	RequestID uint64
	Value     []byte
	TTLMs     int64
	Deleted   uint64
	Keys      []string
	Message   string
}

// EncodeResponse returns the response as packet bytes, with a header.
// If the result is too large, it returns a small TOO_LARGE packet and no error.
// It returns an error if the command, status or number is wrong.
func EncodeResponse(resp Response) ([]byte, error) {
	if err := resp.validate(); err != nil {
		return nil, err
	}

	payload, err := resp.encodePayload()
	if errors.Is(err, ErrTooLarge) || len(payload) > MaxPayloadSize {
		resp = Response{
			Op:        resp.Op,
			Status:    StatusTooLarge,
			RequestID: resp.RequestID,
			Message:   "response exceeds size limit",
		}
		payload, err = resp.encodePayload()
	}
	if err != nil {
		return nil, err
	}

	header := Header{
		Op:         resp.Op,
		Status:     resp.Status,
		Flags:      FlagResponse,
		PayloadLen: uint16(len(payload)),
		RequestID:  resp.RequestID,
	}
	packet := header.appendTo(make([]byte, 0, HeaderSize+len(payload)))

	return append(packet, payload...), nil
}

// DecodeResponse reads packet bytes and returns a Response.
// It returns an error if the packet or its fields are wrong.
// It copies the value, so the caller can reuse the input bytes.
func DecodeResponse(packet []byte) (Response, error) {
	header, err := decodeHeader(packet)
	if err != nil {
		return Response{}, err
	}
	if header.Flags != FlagResponse {
		return Response{}, ErrBadFlags
	}

	resp := Response{Op: header.Op, Status: header.Status, RequestID: header.RequestID}
	if err := resp.validate(); err != nil {
		return Response{}, err
	}

	r := newReader(packet[HeaderSize:])
	if err := resp.readPayload(r); err != nil {
		return Response{}, fmt.Errorf("%s response: %w", resp.Op, err)
	}
	if err := r.finish(); err != nil {
		return Response{}, err
	}
	if err := resp.validate(); err != nil {
		return Response{}, err
	}
	resp.Value = bytes.Clone(resp.Value)

	return resp, nil
}

// hasErrorMessage returns true if the status uses an error message.
// It returns false for OK, NOT_FOUND and NOT_SET.
func (resp Response) hasErrorMessage() bool {
	return resp.Status != StatusOK && resp.Status != StatusNotFound && resp.Status != StatusNotSet
}

// validate checks the command, status and result number.
// It returns nil if they are valid, or an error if they are wrong.
//
//nolint:cyclop
func (resp Response) validate() error {
	if !resp.Status.Valid() {
		return fmt.Errorf("response status 0x%02X: %w", uint8(resp.Status), ErrInvalidArgument)
	}
	if resp.hasErrorMessage() {
		// We must be able to send an error for an unknown command.
		return nil
	}
	if !resp.Op.Valid() {
		return fmt.Errorf("opcode 0x%02X: %w", uint8(resp.Op), ErrUnknownOpCode)
	}

	switch resp.Status {
	case StatusNotFound:
		if resp.Op != OpGet && resp.Op != OpTTL && resp.Op != OpExpire {
			return fmt.Errorf("NOT_FOUND for %s: %w", resp.Op, ErrInvalidArgument)
		}
	case StatusNotSet:
		if resp.Op != OpSetNX {
			return fmt.Errorf("NOT_SET for %s: %w", resp.Op, ErrInvalidArgument)
		}
	case StatusOK:
		return resp.validateNumber()
	}

	return nil
}

// validateNumber checks TTL for TTL responses and Deleted for DEL responses.
// It returns an error if the number is not allowed, or nil if it is allowed.
func (resp Response) validateNumber() error {
	switch resp.Op {
	case OpTTL:
		if resp.TTLMs < -1 || resp.TTLMs > MaxTTLMs {
			return fmt.Errorf("ttl_ms: %w", ErrInvalidArgument)
		}
	case OpDel:
		// DEL takes one key, so Deleted must be 0 or 1.
		if resp.Deleted > 1 {
			return fmt.Errorf("deleted: %w", ErrInvalidArgument)
		}
	}

	return nil
}

// encodePayload returns the result fields as bytes, without a header.
// It returns an error if the command is unknown or a field is too large.
//
//nolint:cyclop
func (resp Response) encodePayload() ([]byte, error) {
	if resp.hasErrorMessage() {
		if len(resp.Message) > MaxErrorLen {
			return nil, fmt.Errorf("message: %w", ErrTooLarge)
		}

		return appendString(nil, resp.Message)
	}
	if resp.Status != StatusOK {
		return nil, nil
	}

	switch resp.Op {
	case OpPing, OpSet, OpSetNX, OpExpire:
		return nil, nil
	case OpGet:
		if len(resp.Value) > MaxValueLen {
			return nil, fmt.Errorf("value: %w", ErrTooLarge)
		}

		return appendBytes(nil, resp.Value)
	case OpTTL:
		return appendI64(nil, resp.TTLMs), nil
	case OpDel:
		return appendU64(nil, resp.Deleted), nil
	case OpKeys:
		return resp.encodeKeys()
	default:
		return nil, ErrUnknownOpCode
	}
}

// encodeKeys returns the key list as bytes, with a zero cursor and a key count.
// It returns ErrTooLarge if a key or the list is too large.
// The whole list must fit in one packet.
func (resp Response) encodeKeys() ([]byte, error) {
	if len(resp.Keys) > math.MaxUint16 {
		return nil, ErrTooLarge
	}
	payload := appendU64(nil, 0) // next_cursor is always zero.
	payload = appendU16(payload, uint16(len(resp.Keys)))
	for _, key := range resp.Keys {
		if len(key) > MaxKeyLen {
			return nil, fmt.Errorf("key: %w", ErrTooLarge)
		}
		// Add 2 bytes for the key length, then the key bytes.
		if len(payload)+2+len(key) > MaxPayloadSize {
			return nil, ErrTooLarge
		}

		var err error
		payload, err = appendString(payload, key)
		if err != nil {
			return nil, err
		}
	}

	return payload, nil
}

// readPayload reads the result fields from r and stores them in resp.
// It returns nil on success, or an error if a field cannot be read or Op is unknown.
func (resp *Response) readPayload(r *reader) error {
	var err error
	if resp.hasErrorMessage() {
		resp.Message, err = r.str(MaxErrorLen)

		return err
	}
	if resp.Status != StatusOK {
		return nil
	}

	switch resp.Op {
	case OpPing, OpSet, OpSetNX, OpExpire:
		// These commands have no data after the header.
	case OpGet:
		resp.Value, err = r.bytes(MaxValueLen)
	case OpTTL:
		resp.TTLMs, err = r.i64()
	case OpDel:
		resp.Deleted, err = r.u64()
	case OpKeys:
		return resp.readKeys(r)
	default:
		return ErrUnknownOpCode
	}

	return err
}

// readKeys reads the key list from r and stores it in resp.Keys.
// It returns an error if the cursor is not zero or a field cannot be read.
// It returns nil if the list was read.
func (resp *Response) readKeys(r *reader) error {
	cursor, err := r.u64()
	if err != nil {
		return err
	}
	if cursor != 0 {
		return fmt.Errorf("pagination is not supported: %w", ErrInvalidArgument)
	}
	count, err := r.u16()
	if err != nil {
		return err
	}
	for range count {
		key, err := r.str(MaxKeyLen)
		if err != nil {
			return err
		}
		resp.Keys = append(resp.Keys, key)
	}

	return nil
}
