package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// UpdateCache is sent by the server to update a session's cache.
type UpdateCache struct {
	// Cache is optional data sent by the downstream server, intended to optimize
	// communication between servers. This data can include shared information that
	// is frequently used across multiple servers and can be used to avoid redundant
	// data fetching (e.g., pre-cached player data or session information).
	// If present, it should be handled by the receiving server to optimize its
	// internal operations or to forward the data as needed.
	Cache []byte
}

// ID ...
func (pk *UpdateCache) ID() uint32 {
	return IDUpdateCache
}

// Marshal ...
func (pk *UpdateCache) Marshal(io protocol.IO) {
	io.ByteSlice(&pk.Cache)
}
