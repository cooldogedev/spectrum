package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// ConnectionResponse is sent by the server in response to a ConnectionRequest.
// It includes unique and deterministic identifiers for the player's connection.
// Both RuntimeID and UniqueID remain unchanged throughout the player's session,
// ensuring consistency across all servers the player transfers or connects to.
type ConnectionResponse struct {
	// RuntimeID is the deterministic runtime ID of the player.
	RuntimeID uint64
	// UniqueID is the deterministic unique ID of the player.
	UniqueID int64
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
