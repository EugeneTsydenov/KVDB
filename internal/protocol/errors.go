package protocol

import "errors"

var (
	// ErrBadMagic: the packet does not start with the protocol magic.
	ErrBadMagic = errors.New("bad magic")
	// ErrUnsupportedVersion: the protocol version is not supported.
	ErrUnsupportedVersion = errors.New("unsupported version")
	// ErrBadLength: framing error. The datagram is shorter than the header,
	// longer than MaxPacketSize, or payload_len does not match its real size.
	ErrBadLength = errors.New("bad length")
	// ErrBadFlags: unknown flag bits, or flags do not match the packet direction.
	ErrBadFlags = errors.New("bad flags")
	// ErrTruncated: the payload ends before the current field is complete.
	ErrTruncated = errors.New("truncated")
	// ErrTrailingBytes: unread bytes remain after the last payload field.
	ErrTrailingBytes = errors.New("trailing bytes")
	// ErrUnknownOpCode: the operation code is not defined.
	ErrUnknownOpCode = errors.New("unknown operation code")
	// ErrTooLarge: a key, value, pattern or message exceeds its limit.
	ErrTooLarge = errors.New("too large")
	// ErrInvalidArgument: a field is well-formed but its value is not allowed.
	ErrInvalidArgument = errors.New("invalid argument")
)

// StatusOf maps a decoding error to the status sent back to the client.
func StatusOf(err error) Status {
	switch {
	case err == nil:
		return StatusOK
	case errors.Is(err, ErrBadMagic),
		errors.Is(err, ErrBadLength),
		errors.Is(err, ErrBadFlags),
		errors.Is(err, ErrTruncated),
		errors.Is(err, ErrTrailingBytes):
		return StatusBadRequest
	case errors.Is(err, ErrUnsupportedVersion):
		return StatusUnsupportedVersion
	case errors.Is(err, ErrUnknownOpCode):
		return StatusUnknownOpCode
	case errors.Is(err, ErrTooLarge):
		return StatusTooLarge
	case errors.Is(err, ErrInvalidArgument):
		return StatusInvalidArgument
	default:
		return StatusInternalError
	}
}

// ShouldReply reports whether the server must answer a packet that failed
// with err. Packets with a bad magic or a framing error are dropped silently.
func ShouldReply(err error) bool {
	return !errors.Is(err, ErrBadMagic) && !errors.Is(err, ErrBadLength)
}
