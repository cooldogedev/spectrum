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
	writeString(buf, pk.Reason)
	writeString(buf, pk.Username)
}

// Decode ...
func (pk *Kick) Decode(buf *bytes.Buffer) {
	pk.Reason = readString(buf)
	pk.Username = readString(buf)
}
