package session

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// Processor is an interface that defines methods for processing incoming and outgoing packets in a session.
type Processor interface {
	// ProcessIncoming determines whether the provided packet, incoming from the server, should be received by the client.
	// It returns true if the packet should be received, otherwise false.
	ProcessIncoming(packet packet.Packet) bool
	// ProcessOutgoing determines whether the provided packet, outgoing from the client, should be sent to the server.
	// It returns true if the packet should be sent, otherwise false.
	ProcessOutgoing(packet packet.Packet) bool
}
