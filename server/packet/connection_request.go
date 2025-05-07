package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// ConnectionRequest is the initial packet sent by the proxy to the server.
// The server responds to this packet with a ConnectionResponse packet.
type ConnectionRequest struct {
	// Addr is the address of the player.
	Addr string
	// ClientData is the player's login.ClientData.
	ClientData []byte
	// IdentityData is the player's login.IdentityData.
	IdentityData []byte
	// ProtocolID is the protocol version identifier of the player.
	ProtocolID int32
	// Cache is optional data sent by the downstream server, intended to optimize
	// communication between servers. This data can include shared information that
	// is frequently used across multiple servers and can be used to avoid redundant
	// data fetching (e.g., pre-cached player data or session information).
	Cache []byte
}

// ID ...
func (pk *ConnectionRequest) ID() uint32 {
	return IDConnectionRequest
}

// Marshal ...
func (pk *ConnectionRequest) Marshal(io protocol.IO) {
	io.String(&pk.Addr)
	io.ByteSlice(&pk.ClientData)
	io.ByteSlice(&pk.IdentityData)
	io.Int32(&pk.ProtocolID)
	io.ByteSlice(&pk.Cache)
}
