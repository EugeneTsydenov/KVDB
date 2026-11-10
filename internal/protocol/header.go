package protocol

import "encoding/binary"

// Header holds the fields from the first 16 bytes of a packet.
type Header struct {
	Op         OpCode
	Status     Status
	Flags      uint8
	PayloadLen uint16
	RequestID  uint64
}

// appendTo adds the header bytes to b and returns the new slice.
func (h *Header) appendTo(b []byte) []byte {
	b = appendU16(b, Magic)
	b = appendU8(b, Version)
	b = appendU8(b, uint8(h.Op))
	b = appendU8(b, uint8(h.Status))
	b = appendU8(b, h.Flags)
	b = appendU16(b, h.PayloadLen)

	return appendU64(b, h.RequestID)
}

// decodeHeader reads and returns the packet header.
// It returns an error if the size, magic, version or payload length is wrong.
func decodeHeader(packet []byte) (Header, error) {
	if len(packet) < HeaderSize || len(packet) > MaxPacketSize {
		return Header{}, ErrBadLength
	}
	if binary.BigEndian.Uint16(packet[:2]) != Magic {
		return Header{}, ErrBadMagic
	}
	if packet[2] != Version {
		return Header{}, ErrUnsupportedVersion
	}

	h := Header{
		Op:         OpCode(packet[3]),
		Status:     Status(packet[4]),
		Flags:      packet[5],
		PayloadLen: binary.BigEndian.Uint16(packet[6:8]),
		RequestID:  binary.BigEndian.Uint64(packet[8:16]),
	}

	if int(h.PayloadLen) != len(packet)-HeaderSize {
		return Header{}, ErrBadLength
	}

	return h, nil
}
