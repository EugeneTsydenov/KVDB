package protocol

import "fmt"

// Status is a response status code. Its value is exactly the byte sent on the wire.
type Status uint8

const (
	StatusOK                 Status = 0x00
	StatusNotFound           Status = 0x01
	StatusNotSet             Status = 0x02
	StatusBadRequest         Status = 0x10
	StatusUnknownOpCode      Status = 0x11
	StatusUnsupportedVersion Status = 0x12
	StatusTooLarge           Status = 0x13
	StatusInvalidArgument    Status = 0x14
	StatusInternalError      Status = 0x7F
)

// Valid reports whether the status is known.
func (s Status) Valid() bool {
	switch s {
	case StatusOK,
		StatusNotFound,
		StatusNotSet,
		StatusBadRequest,
		StatusUnknownOpCode,
		StatusUnsupportedVersion,
		StatusTooLarge,
		StatusInvalidArgument,
		StatusInternalError:
		return true
	default:
		return false
	}
}

//nolint:cyclop
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusNotFound:
		return "NOT_FOUND"
	case StatusNotSet:
		return "NOT_SET"
	case StatusBadRequest:
		return "BAD_REQUEST"
	case StatusUnknownOpCode:
		return "UNKNOWN_OPCODE"
	case StatusUnsupportedVersion:
		return "UNSUPPORTED_VERSION"
	case StatusTooLarge:
		return "TOO_LARGE"
	case StatusInvalidArgument:
		return "INVALID_ARGUMENT"
	case StatusInternalError:
		return "INTERNAL_ERROR"
	default:
		return fmt.Sprintf("Status(0x%02X)", uint8(s))
	}
}
