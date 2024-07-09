package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// Transfer is sent by the server to initiate a server transfer.
type Transfer struct {
	// Addr is the address of the new server.
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
