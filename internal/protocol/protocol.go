package protocol

const (
	// Magic is the value at the start of every KVDB packet.
	Magic = 0x4B56
	// Version is the protocol version this code supports.
	Version = 0x01
	// HeaderSize is the header size in bytes.
	HeaderSize = 16
	// MaxPacketSize is the largest packet size in bytes, with the header.
	MaxPacketSize = 1400
	// MaxPayloadSize is the largest size of the data after the header, in bytes.
	MaxPayloadSize = MaxPacketSize - HeaderSize
	// MaxKeyLen is the largest key size in bytes.
	MaxKeyLen = 256
	// MaxValueLen is the largest value size in bytes.
	MaxValueLen = 1024
	// MaxPatternLen is the largest search pattern size in bytes.
	MaxPatternLen = 256
	// MaxErrorLen is the largest error message size in bytes.
	MaxErrorLen = 256
	// MaxTTLMs is the largest allowed TTL in milliseconds.
	// It can be changed to nanoseconds without going above the int64 limit.
	MaxTTLMs = 9223372036854
	// FlagResponse is the flag for a response. A request uses zero.
	FlagResponse uint8 = 0x01
)
