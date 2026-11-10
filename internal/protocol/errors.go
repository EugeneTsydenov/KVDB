package protocol

import "errors"

var (
	// ErrBadMagic means the first two bytes are not the KVDB magic.
	ErrBadMagic = errors.New("bad magic")
	// ErrUnsupportedVersion means the packet uses a version we do not support.
	ErrUnsupportedVersion = errors.New("unsupported version")
	// ErrBadLength means the packet size or its payload length is wrong.
	ErrBadLength = errors.New("bad length")
	// ErrBadFlags means the flags are wrong for a request or response.
	ErrBadFlags = errors.New("bad flags")
	// ErrTruncated means there are not enough bytes to read a field.
	ErrTruncated = errors.New("truncated")
	// ErrTrailingBytes means extra bytes remain after all fields are read.
	ErrTrailingBytes = errors.New("trailing bytes")
	// ErrUnknownOpCode means the command code is not known.
	ErrUnknownOpCode = errors.New("unknown operation code")
	// ErrTooLarge means a field or the whole packet is too large.
	ErrTooLarge = errors.New("too large")
	// ErrInvalidArgument means a field has a value that is not allowed.
	ErrInvalidArgument = errors.New("invalid argument")
)

// StatusOf returns the response status for err.
// It returns OK for nil and INTERNAL_ERROR for an unknown error.
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

// ShouldReply returns false for bad magic or a wrong packet length.
// It returns true for all other errors and for nil.
func ShouldReply(err error) bool {
	return !errors.Is(err, ErrBadMagic) && !errors.Is(err, ErrBadLength)
}
