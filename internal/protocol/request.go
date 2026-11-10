package protocol

import (
	"bytes"
	"fmt"
	"regexp"
)

// Request holds a command and the data it needs.
// Op tells the code which fields to use.
type Request struct {
	// Command to run.
	Op OpCode
	// ID that the server must copy into the response.
	RequestID uint64
	// Key for GET, SET, SETNX, DEL, TTL and EXPIRE.
	Key []byte
	// Data to store for SET and SETNX.
	Value []byte
	// Key lifetime in milliseconds for SET, SETNX and EXPIRE.
	TTLMs uint64
	// Regex used by KEYS to find keys.
	Pattern string
	// KEYS page cursor. Use zero until pages are supported.
	Cursor uint64
	// KEYS page size. Use zero until pages are supported.
	Limit uint16
}

// EncodeRequest returns the request as packet bytes, with a header.
// It returns an error if a field is wrong or too large.
// For SET and SETNX, TTLMs = 0 means the key has no time limit.
func EncodeRequest(req Request) ([]byte, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	payload, err := req.encodePayload()
	if err != nil {
		return nil, err
	}
	if len(payload) > MaxPayloadSize {
		return nil, ErrTooLarge
	}

	header := Header{
		Op:         req.Op,
		PayloadLen: uint16(len(payload)),
		RequestID:  req.RequestID,
	}
	packet := header.appendTo(make([]byte, 0, HeaderSize+len(payload)))

	return append(packet, payload...), nil
}

// DecodeRequest reads packet bytes and returns a Request.
// It returns an error if the packet or its fields are wrong.
// It copies the key and value, so the caller can reuse the input bytes.
func DecodeRequest(packet []byte) (Request, error) {
	header, err := decodeHeader(packet)
	if err != nil {
		return Request{}, err
	}
	if header.Flags != 0 {
		return Request{}, ErrBadFlags
	}
	if header.Status != StatusOK {
		return Request{}, fmt.Errorf("request status must be OK: %w", ErrInvalidArgument)
	}
	if !header.Op.Valid() {
		return Request{}, fmt.Errorf("opcode 0x%02X: %w", uint8(header.Op), ErrUnknownOpCode)
	}

	req := Request{Op: header.Op, RequestID: header.RequestID}
	r := newReader(packet[HeaderSize:])
	if err := req.readPayload(r); err != nil {
		return Request{}, fmt.Errorf("%s payload: %w", req.Op, err)
	}
	if err := r.finish(); err != nil {
		return Request{}, err
	}
	if err := req.validate(); err != nil {
		return Request{}, err
	}
	req.Key = bytes.Clone(req.Key)
	req.Value = bytes.Clone(req.Value)

	return req, nil
}

// validate checks the command and the fields it uses.
// It returns nil if they are valid, or an error if they are wrong.
//
//nolint:cyclop
func (req Request) validate() error {
	switch req.Op {
	case OpPing:
		return nil

	case OpGet, OpDel, OpTTL:
		return validateKey(req.Key)

	case OpSet, OpSetNX:
		if err := validateKey(req.Key); err != nil {
			return err
		}
		if len(req.Value) > MaxValueLen {
			return fmt.Errorf("value: %w", ErrTooLarge)
		}
		if req.TTLMs > MaxTTLMs {
			return fmt.Errorf("ttl_ms: %w", ErrInvalidArgument)
		}

		return nil

	case OpExpire:
		if err := validateKey(req.Key); err != nil {
			return err
		}
		if req.TTLMs == 0 || req.TTLMs > MaxTTLMs {
			return fmt.Errorf("ttl_ms: %w", ErrInvalidArgument)
		}

		return nil

	case OpKeys:
		if len(req.Pattern) > MaxPatternLen {
			return fmt.Errorf("pattern: %w", ErrTooLarge)
		}
		if _, err := regexp.Compile(req.Pattern); err != nil {
			return fmt.Errorf("pattern: %w: %w", ErrInvalidArgument, err)
		}

		return nil

	default:
		return fmt.Errorf("opcode 0x%02X: %w", uint8(req.Op), ErrUnknownOpCode)
	}
}

// validateKey returns ErrTooLarge if the key is too long, or nil if it fits.
func validateKey(key []byte) error {
	if len(key) > MaxKeyLen {
		return fmt.Errorf("key: %w", ErrTooLarge)
	}

	return nil
}

// encodePayload returns the command fields as bytes, without a header.
// It returns an error if the command is unknown or a field cannot be written.
func (req Request) encodePayload() ([]byte, error) {
	var payload []byte
	var err error

	switch req.Op {
	case OpPing:
		// PING has no fields.

	case OpGet, OpDel, OpTTL:
		payload, err = appendBytes(payload, req.Key)

	case OpSet, OpSetNX:
		payload, err = appendBytes(payload, req.Key)
		if err != nil {
			return nil, err
		}
		payload, err = appendBytes(payload, req.Value)
		if err != nil {
			return nil, err
		}
		payload = appendU64(payload, req.TTLMs)

	case OpExpire:
		payload, err = appendBytes(payload, req.Key)
		if err != nil {
			return nil, err
		}
		payload = appendU64(payload, req.TTLMs)

	case OpKeys:
		payload, err = appendString(payload, req.Pattern)
		if err != nil {
			return nil, err
		}
		payload = appendU64(payload, req.Cursor)
		payload = appendU16(payload, req.Limit)

	default:
		return nil, fmt.Errorf("opcode 0x%02X: %w", uint8(req.Op), ErrUnknownOpCode)
	}

	return payload, err
}

// readPayload reads the command fields from r and stores them in req.
// It returns nil on success, or an error if a field cannot be read or Op is unknown.
//
//nolint:cyclop
func (req *Request) readPayload(r *reader) error {
	var err error

	switch req.Op {
	case OpPing:
		// PING has no fields.

	case OpGet, OpDel, OpTTL:
		req.Key, err = r.bytes(MaxKeyLen)

	case OpSet, OpSetNX:
		req.Key, err = r.bytes(MaxKeyLen)
		if err != nil {
			return fmt.Errorf("key: %w", err)
		}
		req.Value, err = r.bytes(MaxValueLen)
		if err != nil {
			return fmt.Errorf("value: %w", err)
		}
		req.TTLMs, err = r.u64()

	case OpExpire:
		req.Key, err = r.bytes(MaxKeyLen)
		if err != nil {
			return fmt.Errorf("key: %w", err)
		}
		req.TTLMs, err = r.u64()

	case OpKeys:
		req.Pattern, err = r.str(MaxPatternLen)
		if err != nil {
			return fmt.Errorf("pattern: %w", err)
		}
		req.Cursor, err = r.u64()
		if err != nil {
			return fmt.Errorf("cursor: %w", err)
		}
		req.Limit, err = r.u16()

	default:
		return fmt.Errorf("opcode 0x%02X: %w", uint8(req.Op), ErrUnknownOpCode)
	}

	return err
}
