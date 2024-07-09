package packet

import "bytes"

// ConnectionRequest is sent by clients to connect and authenticate with the service using a token.
// The service responds to this request with a ConnectionResponse.
type ConnectionRequest struct {
	// Token is the client's token which is used for authorization.
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
