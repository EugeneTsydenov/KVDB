package storage

type ValueType uint8

const (
	TypeString ValueType = iota + 1
)

type Value struct {
	Type ValueType
	Data []byte
}

func NewStringValue(data []byte) Value {
	return Value{
		Type: TypeString,
		Data: append([]byte(nil), data...),
	}
}

func (v Value) Clone() Value {
	return Value{
		Type: v.Type,
		Data: append([]byte(nil), v.Data...),
	}
}
