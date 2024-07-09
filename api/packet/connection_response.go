package packet

import (
	"bytes"
	"encoding/binary"
)

const (
	ResponseSuccess = iota
	ResponseUnauthorized
	ResponseFail
)

// ConnectionResponse represents the response sent by the API service
// in reply to a ConnectionRequest.
type ConnectionResponse struct {
	// Response indicates the status of the connection request.
	Response uint8
}

// ID ...
func (pk *ConnectionResponse) ID() uint32 {
	return IDConnectionResponse
}

// Encode ...
func (pk *ConnectionResponse) Encode(buf *bytes.Buffer) {
	_ = binary.Write(buf, binary.LittleEndian, pk.Response)
}

// Decode ...
func (pk *ConnectionResponse) Decode(buf *bytes.Buffer) {
	_ = binary.Read(buf, binary.LittleEndian, pk.Response)
}
