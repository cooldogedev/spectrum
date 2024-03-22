package packet

import "bytes"

type Kick struct {
	Reason   string
	Username string
}

// ID ...
func (k *Kick) ID() uint32 {
	return IDKick
}

// Encode ...
func (k *Kick) Encode(buf *bytes.Buffer) {
	writeString(buf, k.Reason)
	writeString(buf, k.Username)
}

// Decode ...
func (k *Kick) Decode(buf *bytes.Buffer) {
	k.Reason = readString(buf)
	k.Username = readString(buf)
}
