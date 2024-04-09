package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type ConnectionRequest struct {
	Addr  string
	Token string

	ClientData   []byte
	IdentityData []byte
}

// ID ...
func (pk *ConnectionRequest) ID() uint32 {
	return IDConnectionRequest
}

// Marshal ...
func (pk *ConnectionRequest) Marshal(io protocol.IO) {
	io.String(&pk.Addr)
	io.String(&pk.Token)

	io.ByteSlice(&pk.ClientData)
	io.ByteSlice(&pk.IdentityData)
}
