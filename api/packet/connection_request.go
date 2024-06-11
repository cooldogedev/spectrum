package packet

import "bytes"

type ConnectionRequest struct {
	Token string
}

// ID ...
func (pk *ConnectionRequest) ID() uint32 {
	return IDConnectionRequest
}

// Encode ...
func (pk *ConnectionRequest) Encode(buf *bytes.Buffer) {
	WriteString(buf, pk.Token)
}

// Decode ...
func (pk *ConnectionRequest) Decode(buf *bytes.Buffer) {
	pk.Token = ReadString(buf)
}
