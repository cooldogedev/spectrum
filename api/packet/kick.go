package packet

import "bytes"

type Kick struct {
	Reason   string
	Username string
}

// ID ...
func (pk *Kick) ID() uint32 {
	return IDKick
}

// Encode ...
func (pk *Kick) Encode(buf *bytes.Buffer) {
	WriteString(buf, pk.Reason)
	WriteString(buf, pk.Username)
}

// Decode ...
func (pk *Kick) Decode(buf *bytes.Buffer) {
	pk.Reason = ReadString(buf)
	pk.Username = ReadString(buf)
}
