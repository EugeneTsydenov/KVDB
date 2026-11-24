package protocol

import "errors"

// StatusOf maps local validation errors to response status codes.
func StatusOf(err error) Status {
	switch {
	case err == nil:
		return StatusOK
	case errors.Is(err, ErrUnsupportedVersion):
		return StatusUnsupportedVersion
	case errors.Is(err, ErrUnknownOpcode):
		return StatusUnknownOpCode
	case errors.Is(err, ErrPacketTooLarge):
		return StatusTooLarge
	case errors.Is(err, ErrInvalidArgument):
		return StatusInvalidArgument
	case errors.Is(err, ErrInvalidPacket):
		return StatusBadRequest
	default:
		return StatusInternalError
	}
}

// ShouldReply identifies validation failures that can be reported to a client.
// The caller must also use DecodeHeader to verify reply metadata and datagram size.
func ShouldReply(err error) bool {
	return errors.Is(err, ErrUnsupportedVersion) ||
		errors.Is(err, ErrUnknownOpcode) ||
		errors.Is(err, ErrPacketTooLarge) ||
		errors.Is(err, ErrInvalidArgument) ||
		errors.Is(err, ErrInvalidPacket)
}
