package protocol

import (
	"fmt"

	"kvdb/internal/protocol/pb"
)

// OpCode is code for a command, such as GET or SET.
type OpCode int32

const (
	OpUnspecified OpCode = iota
	OpPing
	OpGet
	OpSet
	OpSetNX
	OpDel
	OpTTL
	OpExpire
	OpKeys
)

// Valid returns true if the command code is known. Otherwise, it returns false.
func (o OpCode) Valid() bool {
	switch o {
	case OpPing, OpGet, OpSet, OpSetNX, OpDel, OpTTL, OpExpire, OpKeys:
		return true
	default:
		return false
	}
}

// String returns the command name, or a name like "OpCode(1)" for an unknown code.
func (o OpCode) String() string {
	switch o {
	case OpPing:
		return "PING"
	case OpGet:
		return "GET"
	case OpSet:
		return "SET"
	case OpSetNX:
		return "SETNX"
	case OpDel:
		return "DEL"
	case OpTTL:
		return "TTL"
	case OpExpire:
		return "EXPIRE"
	case OpKeys:
		return "KEYS"
	default:
		return fmt.Sprintf("OpCode(%d)", int(o))
	}
}

func (o OpCode) ToPbOperation() pb.Operation {
	return pb.Operation(o)
}

func OpCodeFromPb(op pb.Operation) (OpCode, error) {
	switch op {
	case pb.Operation_OPERATION_PING:
		return OpPing, nil
	case pb.Operation_OPERATION_GET:
		return OpGet, nil
	case pb.Operation_OPERATION_SET:
		return OpSet, nil
	case pb.Operation_OPERATION_SETNX:
		return OpSetNX, nil
	case pb.Operation_OPERATION_DEL:
		return OpDel, nil
	case pb.Operation_OPERATION_TTL:
		return OpTTL, nil
	case pb.Operation_OPERATION_EXPIRE:
		return OpExpire, nil
	case pb.Operation_OPERATION_KEYS:
		return OpKeys, nil
	default:
		return 0, ErrUnknownOpcode
	}
}
