package packet

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type ConnectionRequest struct {
	Addr         string
	ClientData   []byte
	IdentityData []byte
}

func (pk *ConnectionRequest) ID() uint32 {
	return IDConnectionRequest
}

func (pk *ConnectionRequest) Marshal(io protocol.IO) {
	io.String(&pk.Addr)
	io.ByteSlice(&pk.ClientData)
	io.ByteSlice(&pk.IdentityData)
}
