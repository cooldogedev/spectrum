package session

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/spectrum-proxy/spectrum/event"
)

type Handler interface {
	// HandleIncoming handle incoming packets to the session
	HandleIncoming(ctx *event.Context, pk packet.Packet)
	// HandleOutgoing handle outgoing packets from the session
	HandleOutgoing(ctx *event.Context, pk packet.Packet)
}

type NoopHandler struct{}

func (NoopHandler) HandleIncoming(*event.Context, packet.Packet) {}
func (NoopHandler) HandleOutgoing(*event.Context, packet.Packet) {}
