package packet

import "bytes"

// Transfer is sent by the client to initiate the transfer of a specific player to another server.
type Transfer struct {
	// Addr is the address of the new server.
	Addr string
	// Username is the username of the player to be transferred.
	Username string
}

// ID ...
func (pk *Transfer) ID() uint32 {
	return IDTransfer
}

// Encode ...
func (pk *Transfer) Encode(buf *bytes.Buffer) {
	WriteString(buf, pk.Addr)
	WriteString(buf, pk.Username)
}

// Decode ...
func (pk *Transfer) Decode(buf *bytes.Buffer) {
	pk.Addr = ReadString(buf)
	pk.Username = ReadString(buf)
}
