package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type ConnectionResponse struct {
	RuntimeID uint64
	UniqueID  int64
}

// ID ...
func (pk *ConnectionResponse) ID() uint32 {
	return IDConnectionResponse
}

// Marshal ...
func (pk *ConnectionResponse) Marshal(io protocol.IO) {
	io.Varuint64(&pk.RuntimeID)
	io.Varint64(&pk.UniqueID)
}
