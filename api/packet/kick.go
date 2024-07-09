package packet

import "bytes"

// Kick is sent by the client to initiate the removal of a specific player from the proxy.
type Kick struct {
	// Reason is the reason displayed in the disconnection screen for the kick.
	Reason string
	// Username is the username of the player to be kicked.
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
