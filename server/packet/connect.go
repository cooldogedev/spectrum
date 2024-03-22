package packet

import (
	"encoding/json"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

type Connect struct {
	Addr     string
	EntityID int64

	ClientData   login.ClientData
	IdentityData login.IdentityData
}

func (pk *Connect) ID() uint32 {
	return IDConnect
}

func (pk *Connect) Marshal(io protocol.IO) {
	clientData, _ := json.Marshal(pk.ClientData)
	identityData, _ := json.Marshal(pk.IdentityData)

	io.String(&pk.Addr)
	io.Varint64(&pk.EntityID)

	io.ByteSlice(&clientData)
	io.ByteSlice(&identityData)
}
