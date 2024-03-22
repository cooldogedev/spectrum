package packet

import "bytes"

type Transfer struct {
	Addr     string
	Username string
}

// ID ...
func (t *Transfer) ID() uint32 {
	return IDTransfer
}

// Encode ...
func (t *Transfer) Encode(buf *bytes.Buffer) {
	writeString(buf, t.Addr)
	writeString(buf, t.Username)
}

// Decode ...
func (t *Transfer) Decode(buf *bytes.Buffer) {
	t.Addr = readString(buf)
	t.Username = readString(buf)
}
