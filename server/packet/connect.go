package packet

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type Connect struct {
	Addr         string
	EntityID     int64
	ClientData   []byte
	IdentityData []byte
}

func (pk *Connect) ID() uint32 {
	return IDConnect
}

func (pk *Connect) Marshal(io protocol.IO) {
	io.String(&pk.Addr)
	io.Varint64(&pk.EntityID)
	io.ByteSlice(&pk.ClientData)
	io.ByteSlice(&pk.IdentityData)
}
