package protocol

const (
	Magic                = "protocol"
	Version              = "0.1"
	HeaderSize           = 16
	MaxPacketSize        = 1400
	MaxPayloadSize       = MaxPacketSize - HeaderSize
	MaxKeyLen            = 256
	MaxErrorLen          = 256
	MaxTTLMs             = 9223372036854
	FlagResponse   uint8 = 0x01
)
