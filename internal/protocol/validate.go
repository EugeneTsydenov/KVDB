package protocol

import (
	"errors"
	"regexp"
	"unicode/utf8"

	"kvdb/internal/protocol/pb"
)

const (
	ProtocolVersion = 1

	MaxKeyLen     = 256
	MaxValueLen   = 1024
	MaxPatternLen = 256
	MaxErrorLen   = 256

	MaxTTLMs uint64 = 9223372036854
)

var (
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrUnknownOpcode      = errors.New("unknown opcode")
	ErrInvalidArgument    = errors.New("invalid argument")
)

// Validate checks the packet envelope and its body.
func Validate(packet *pb.Packet) error {
	if packet == nil {
		return ErrInvalidPacket
	}

	if packet.GetVersion() != ProtocolVersion {
		return ErrUnsupportedVersion
	}

	if packet.GetClientId() == 0 ||
		packet.GetRequestId() == 0 ||
		packet.GetSequence() == 0 {
		return ErrInvalidPacket
	}

	switch body := packet.GetBody().(type) {
	case *pb.Packet_Request:
		return validateRequest(packet.GetOp(), body.Request)

	case *pb.Packet_Response:
		return validateResponse(packet.GetOp(), body.Response)

	case *pb.Packet_Ack:
		return validateAck(packet.GetOp(), body.Ack)

	default:
		return ErrInvalidPacket
	}
}

// validateRequest checks command arguments.
//
//nolint:gocognit,gocyclo,cyclop,funlen
func validateRequest(op pb.Operation, req *pb.Request) error {
	if req == nil {
		return ErrInvalidPacket
	}

	if !validOperation(op) {
		return ErrUnknownOpcode
	}

	if len(req.GetKey()) > MaxKeyLen ||
		len(req.GetValue()) > MaxValueLen ||
		len(req.GetPattern()) > MaxPatternLen {
		return ErrPacketTooLarge
	}

	if req.GetTtlMs() > MaxTTLMs {
		return ErrInvalidArgument
	}

	switch op {
	case pb.Operation_OPERATION_PING:
		if !emptyRequest(req) {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_GET,
		pb.Operation_OPERATION_DEL,
		pb.Operation_OPERATION_TTL:

		if !validKey(req.GetKey()) {
			return ErrInvalidArgument
		}

		if len(req.GetValue()) != 0 ||
			req.GetTtlMs() != 0 ||
			req.GetPattern() != "" ||
			req.GetCursor() != 0 ||
			req.GetLimit() != 0 {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_SET,
		pb.Operation_OPERATION_SETNX:

		if !validKey(req.GetKey()) {
			return ErrInvalidArgument
		}

		// Empty values and TTL = 0 are allowed.
		if req.GetPattern() != "" ||
			req.GetCursor() != 0 ||
			req.GetLimit() != 0 {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_EXPIRE:
		if !validKey(req.GetKey()) {
			return ErrInvalidArgument
		}

		if req.GetTtlMs() == 0 {
			return ErrInvalidArgument
		}

		if len(req.GetValue()) != 0 ||
			req.GetPattern() != "" ||
			req.GetCursor() != 0 ||
			req.GetLimit() != 0 {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_KEYS:
		if len(req.GetKey()) != 0 ||
			len(req.GetValue()) != 0 ||
			req.GetTtlMs() != 0 {
			return ErrInvalidPacket
		}

		if req.GetCursor() != 0 ||
			req.GetLimit() != 0 {
			return ErrInvalidArgument
		}

		if !utf8.ValidString(req.GetPattern()) {
			return ErrInvalidArgument
		}

		if _, err := regexp.Compile(req.GetPattern()); err != nil {
			return ErrInvalidArgument
		}
	}

	return nil
}

// validateResponse checks statuses and command results.
//
//nolint:gocognit,gocyclo,cyclop,funlen
func validateResponse(op pb.Operation, res *pb.Response) error {
	if res == nil {
		return ErrInvalidPacket
	}

	if len(res.GetMessage()) > MaxErrorLen {
		return ErrPacketTooLarge
	}

	if !utf8.ValidString(res.GetMessage()) {
		return ErrInvalidPacket
	}

	status := res.GetStatus()

	if !validStatus(status) {
		return ErrInvalidPacket
	}

	// Error responses must not contain command result data.
	if isErrorStatus(status) {
		if !validOperation(op) &&
			status != pb.Status_STATUS_UNKNOWN_OPCODE &&
			status != pb.Status_STATUS_UNSUPPORTED_VERSION {
			return ErrUnknownOpcode
		}

		if !emptyResponseData(res) {
			return ErrInvalidPacket
		}

		return nil
	}

	if !validOperation(op) {
		return ErrUnknownOpcode
	}

	// Successful responses must not contain an error message.
	if res.GetMessage() != "" {
		return ErrInvalidPacket
	}

	switch status {
	case pb.Status_STATUS_NOT_FOUND:
		if op != pb.Operation_OPERATION_GET &&
			op != pb.Operation_OPERATION_TTL &&
			op != pb.Operation_OPERATION_EXPIRE {
			return ErrInvalidPacket
		}

		if !emptyResponseData(res) {
			return ErrInvalidPacket
		}

		return nil

	case pb.Status_STATUS_NOT_SET:
		if op != pb.Operation_OPERATION_SETNX {
			return ErrInvalidPacket
		}

		if !emptyResponseData(res) {
			return ErrInvalidPacket
		}

		return nil

	case pb.Status_STATUS_OK:
		// Validate result fields below.

	default:
		return ErrInvalidPacket
	}

	switch op {
	case pb.Operation_OPERATION_PING,
		pb.Operation_OPERATION_SET,
		pb.Operation_OPERATION_SETNX,
		pb.Operation_OPERATION_EXPIRE:

		if !emptyResponseData(res) {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_GET:
		if len(res.GetValue()) > MaxValueLen {
			return ErrPacketTooLarge
		}

		if res.GetTtlMs() != 0 ||
			res.GetDeleted() != 0 ||
			len(res.GetKeys()) != 0 {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_DEL:
		if res.GetDeleted() > 1 {
			return ErrInvalidPacket
		}

		if len(res.GetValue()) != 0 ||
			res.GetTtlMs() != 0 ||
			len(res.GetKeys()) != 0 {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_TTL:
		if res.GetTtlMs() < -1 ||
			res.GetTtlMs() > int64(MaxTTLMs) {
			return ErrInvalidArgument
		}

		if len(res.GetValue()) != 0 ||
			res.GetDeleted() != 0 ||
			len(res.GetKeys()) != 0 {
			return ErrInvalidPacket
		}

	case pb.Operation_OPERATION_KEYS:
		if len(res.GetValue()) != 0 ||
			res.GetTtlMs() != 0 ||
			res.GetDeleted() != 0 {
			return ErrInvalidPacket
		}

		for _, key := range res.GetKeys() {
			if len(key) > MaxKeyLen {
				return ErrPacketTooLarge
			}

			if !validKey(key) {
				return ErrInvalidPacket
			}
		}
	}

	return nil
}

// validateAck checks the acknowledgement message.
func validateAck(op pb.Operation, ack *pb.Ack) error {
	if ack == nil {
		return ErrInvalidPacket
	}

	if !validOperation(op) {
		return ErrUnknownOpcode
	}

	return nil
}

// validOperation checks whether the operation is supported.
func validOperation(op pb.Operation) bool {
	switch op {
	case pb.Operation_OPERATION_PING,
		pb.Operation_OPERATION_GET,
		pb.Operation_OPERATION_SET,
		pb.Operation_OPERATION_SETNX,
		pb.Operation_OPERATION_DEL,
		pb.Operation_OPERATION_TTL,
		pb.Operation_OPERATION_EXPIRE,
		pb.Operation_OPERATION_KEYS:
		return true

	default:
		return false
	}
}

// validStatus checks whether the response status is defined.
func validStatus(status pb.Status) bool {
	switch status {
	case pb.Status_STATUS_OK,
		pb.Status_STATUS_NOT_FOUND,
		pb.Status_STATUS_NOT_SET,
		pb.Status_STATUS_BAD_REQUEST,
		pb.Status_STATUS_UNKNOWN_OPCODE,
		pb.Status_STATUS_UNSUPPORTED_VERSION,
		pb.Status_STATUS_TOO_LARGE,
		pb.Status_STATUS_INVALID_ARGUMENT,
		pb.Status_STATUS_REQUEST_CONFLICT,
		pb.Status_STATUS_SERVER_BUSY,
		pb.Status_STATUS_INTERNAL_ERROR:
		return true

	default:
		return false
	}
}

// isErrorStatus reports whether the status represents an error.
func isErrorStatus(status pb.Status) bool {
	switch status {
	case pb.Status_STATUS_BAD_REQUEST,
		pb.Status_STATUS_UNKNOWN_OPCODE,
		pb.Status_STATUS_UNSUPPORTED_VERSION,
		pb.Status_STATUS_TOO_LARGE,
		pb.Status_STATUS_INVALID_ARGUMENT,
		pb.Status_STATUS_REQUEST_CONFLICT,
		pb.Status_STATUS_SERVER_BUSY,
		pb.Status_STATUS_INTERNAL_ERROR:
		return true

	default:
		return false
	}
}

// validKey checks the key length.
func validKey(key []byte) bool {
	return len(key) > 0 && len(key) <= MaxKeyLen
}

// emptyRequest reports whether all request arguments are empty.
func emptyRequest(req *pb.Request) bool {
	return len(req.GetKey()) == 0 &&
		len(req.GetValue()) == 0 &&
		req.GetTtlMs() == 0 &&
		req.GetPattern() == "" &&
		req.GetCursor() == 0 &&
		req.GetLimit() == 0
}

// emptyResponseData reports whether the response has no result data.
func emptyResponseData(res *pb.Response) bool {
	return len(res.GetValue()) == 0 &&
		res.GetTtlMs() == 0 &&
		res.GetDeleted() == 0 &&
		len(res.GetKeys()) == 0
}
