package protocol

import "fmt"

// OpCode is a command code. Its value is exactly the byte sent on the wire.
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

// Valid reports whether the code is a known command.
func (o OpCode) Valid() bool {
	switch o {
	case OpPing, OpGet, OpSet, OpSetNX, OpDel, OpTTL, OpExpire, OpKeys:
		return true
	default:
		return false
	}
}

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
