package packet

import "bytes"

type Transfer struct {
	Addr     string
	Username string
}

// ID ...
func (pk *Transfer) ID() uint32 {
	return IDTransfer
}

// Encode ...
func (pk *Transfer) Encode(buf *bytes.Buffer) {
	writeString(buf, pk.Addr)
	writeString(buf, pk.Username)
}

// Decode ...
func (pk *Transfer) Decode(buf *bytes.Buffer) {
	pk.Addr = readString(buf)
	pk.Username = readString(buf)
}
