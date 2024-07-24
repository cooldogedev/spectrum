package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// ConnectionRequest is the initial packet sent by the proxy to the server.
// The server responds to this packet with a ConnectionResponse packet.
type ConnectionRequest struct {
	// Addr is the address of the player.
	Addr string
	// Token is the proxy's token that's used for authorization by the server.
	Token string
	// ClientData is the player's login.ClientData.
	ClientData []byte
	// IdentityData is the player's login.IdentityData.
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
