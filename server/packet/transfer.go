package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type Transfer struct {
	Addr string
}

// ID ...
func (pk *Transfer) ID() uint32 {
	return IDTransfer
}

// Marshal ...
func (pk *Transfer) Marshal(io protocol.IO) {
	io.String(&pk.Addr)
}
