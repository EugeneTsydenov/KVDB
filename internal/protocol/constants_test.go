package protocol

import (
	"testing"
)

func TestProtocolLimits(t *testing.T) {
	t.Parallel()

	if HeaderSize+MaxPayloadSize != MaxPacketSize {
		t.Fatalf("header + payload = %d, want packet size %d", HeaderSize+MaxPayloadSize, MaxPacketSize)
	}
	if MaxPacketSize != 1400 || HeaderSize != 16 {
		t.Fatalf("unexpected packet limits: packet=%d header=%d", MaxPacketSize, HeaderSize)
	}
	if MaxKeyLen <= 0 || MaxValueLen <= 0 || MaxPatternLen <= 0 || MaxErrorLen <= 0 {
		t.Fatal("field limits must be positive")
	}
	if MaxTTLMs <= 0 {
		t.Fatal("MaxTTLMs must be positive")
	}
}
