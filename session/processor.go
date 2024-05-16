package session

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// Processor is an interface defining methods for handling incoming and outgoing packets within a session.
type Processor interface {
	// ProcessServer determines whether the provided packet, originating from the server, should be forwarded to the client.
	// It returns true if the packet should be forwarded, otherwise false.
	ProcessServer(packet packet.Packet) bool
	// ProcessClient determines whether the provided packet, originating from the client, should be forwarded to the server.
	// It returns true if the packet should be forwarded, otherwise false.
	ProcessClient(packet packet.Packet) bool
}
