package protocol

import "fmt"

// OpCode is the one-byte code for a command, such as GET or SET.
type OpCode uint8

const (
	OpPing   OpCode = 0x01
	OpGet    OpCode = 0x02
	OpSet    OpCode = 0x03
	OpSetNX  OpCode = 0x04
	OpDel    OpCode = 0x05
	OpTTL    OpCode = 0x06
	OpExpire OpCode = 0x07
	OpKeys   OpCode = 0x08
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

// String returns the command name, or a name like "OpCode(0xFF)" for an unknown code.
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
		return fmt.Sprintf("OpCode(0x%02X)", uint8(o))
	}
}
