package session

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// Processor is an interface defining methods for handling events such as packet sending/receiving, transfers and disconnection.
type Processor interface {
	// ProcessServer determines whether the provided packet, originating from the server, should be forwarded to the client.
	// It returns true if the packet should be forwarded, otherwise false.
	ProcessServer(packet packet.Packet) bool
	// ProcessClient determines whether the provided packet, originating from the client, should be forwarded to the server.
	// It returns true if the packet should be forwarded, otherwise false.
	ProcessClient(packet packet.Packet) bool

	// ProcessPreTransfer is called before transferring the client to a different server.
	// It can be used for pre-processing tasks before the client transfer starts.
	// The 'addr' parameter represents the address of the target server.
	// It returns true if the transfer process should proceed, otherwise false.
	ProcessPreTransfer(addr string) bool
	// ProcessPostTransfer is called after transferring the client to a different server.
	// It can be used for post-processing tasks after the client transfer completes.
	// The 'addr' parameter represents the address of the target server.
	// Although this method returns a boolean value, it is not currently used and exists for the sake of completeness.
	ProcessPostTransfer(addr string) bool

	// ProcessDisconnection is called when the client disconnects from the server.
	// It can be used for post-processing tasks after the disconnection occurs.
	// Although this method returns a boolean value, it is not currently used and exists for the sake of completeness.
	ProcessDisconnection() bool
}
