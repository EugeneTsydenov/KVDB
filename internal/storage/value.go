package storage

// ValueType represents the type of data stored inside a Value.
type ValueType uint8

const (
	// TypeString represents a standard text or binary string value.
	TypeString ValueType = iota
)

// Value represents a stored data item, containing its type and raw bytes.
type Value struct {
	Type ValueType
	Data []byte
}

// NewStringValue creates a new Value of type string from raw byte data.
func NewStringValue(data []byte) Value {
	return Value{
		Type: TypeString,
		Data: data,
	}
}

// Clone returns a deep copy of the Value.
// This prevents data races when sharing byte slices across goroutines.
func (v Value) Clone() Value {
	return Value{
		Type: v.Type,
		Data: append([]byte(nil), v.Data...),
	}
}
