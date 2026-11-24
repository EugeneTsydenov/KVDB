package protocol

import (
	"fmt"

	"kvdb/internal/protocol/pb"
)

// Status is the result code in a response.
type Status int32

const (
	StatusUnspecified        Status = 0
	StatusOK                 Status = 1
	StatusNotFound           Status = 2
	StatusNotSet             Status = 3
	StatusBadRequest         Status = 16
	StatusUnknownOpCode      Status = 17
	StatusUnsupportedVersion Status = 18
	StatusTooLarge           Status = 19
	StatusInvalidArgument    Status = 20
	StatusRequestConflict    Status = 21
	StatusServerBusy         Status = 22
	StatusInternalError      Status = 127
)

// Valid returns true if the status code is known. Otherwise, it returns false.
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
		StatusRequestConflict,
		StatusServerBusy,
		StatusInternalError:
		return true
	default:
		return false
	}
}

// String returns the status name, or a name like "Status(1)" for an unknown code.
//
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
	case StatusRequestConflict:
		return "REQUEST_CONFLICT"
	case StatusServerBusy:
		return "SERVER_BUSY"
	case StatusInternalError:
		return "INTERNAL_ERROR"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

func (s Status) ToPbStatus() pb.Status {
	return pb.Status(s)
}

func StatusFromPb(status pb.Status) (Status, error) {
	s := Status(status)
	if !s.Valid() {
		return 0, ErrInvalidPacket
	}

	return s, nil
}
