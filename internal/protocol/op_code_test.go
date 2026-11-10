package protocol

import (
	"testing"
)

func TestOpCodeValues(t *testing.T) {
	t.Parallel()

	opcodes := []OpCode{OpPing, OpGet, OpSet, OpSetNX, OpDel, OpTTL, OpExpire, OpKeys}
	for _, op := range opcodes {
		if !op.Valid() {
			t.Errorf("%v should be valid", op)
		}
		if op.String() == "" {
			t.Errorf("%d has an empty string representation", op)
		}
	}
	if OpCode(0xFF).Valid() || OpCode(0xFF).String() != "OpCode(0xFF)" {
		t.Error("unknown opcode handling is incorrect")
	}
}
