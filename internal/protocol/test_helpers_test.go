package protocol

func packetForTest(op OpCode, status Status, flags uint8, payload []byte) []byte {
	header := Header{
		Op:         op,
		Status:     status,
		Flags:      flags,
		PayloadLen: uint16(len(payload)),
		RequestID:  42,
	}
	packet := header.appendTo(nil)
	return append(packet, payload...)
}

func appendBytesForTest(data []byte) []byte {
	encoded, err := appendBytes(nil, data)
	if err != nil {
		panic(err)
	}
	return encoded
}
