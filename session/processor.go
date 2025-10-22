package session

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Context represents the context of an action. It holds the state of whether the action has been canceled.
type Context struct {
	canceled bool
}

// NewContext returns a new context.
func NewContext() *Context {
	return &Context{}
}

// Cancel marks the context as canceled. This function is used to stop further processing of an action.
func (c *Context) Cancel() {
	c.canceled = true
}

// Cancelled returns whether the context has been cancelled.
func (c *Context) Cancelled() bool {
	return c.canceled
}

// Processor defines methods for processing various actions within a proxy session.
type Processor interface {
	// ProcessStartGame is called only once during the login sequence.
	ProcessStartGame(ctx *Context, data *minecraft.GameData)
	// ProcessServer is called before forwarding the server-sent packets to the client.
	ProcessServer(ctx *Context, pk *packet.Packet)
	// ProcessServerEncoded is called before forwarding the server-sent packets to the client.
	ProcessServerEncoded(ctx *Context, pk *[]byte)
	// ProcessClient is called before forwarding the client-sent packets to the server.
	ProcessClient(ctx *Context, pk *packet.Packet)
	// ProcessClientEncoded is called before forwarding the client-sent packets to the server.
	ProcessClientEncoded(ctx *Context, pk *[]byte)
	// ProcessFlush is called before flushing the player's minecraft.Conn buffer in response to a downstream server request.
	ProcessFlush(ctx *Context)
	// ProcessDiscover is called to determine the primary server to send the player to.
	ProcessDiscover(ctx *Context, target *string)
	// ProcessDiscoverFallback is called to determine the fallback server to send the player to.
	ProcessDiscoverFallback(ctx *Context, target *string)
	// ProcessPreTransfer is called before transferring the player to a different server.
	ProcessPreTransfer(ctx *Context, origin *string, target *string)
	// ProcessTransferFailure is called when the player transfer to a different server fails.
	ProcessTransferFailure(ctx *Context, origin *string, target *string)
	// ProcessPostTransfer is called after transferring the player to a different server.
	ProcessPostTransfer(ctx *Context, origin *string, target *string)
	// ProcessCache is called before updating the session's cache.
	ProcessCache(ctx *Context, new *[]byte)
	// ProcessDisconnection is called when the player disconnects from the proxy.
	ProcessDisconnection(ctx *Context, message *string)
}

// NopProcessor is a no-operation implementation of the Processor interface.
type NopProcessor struct{}

// Ensure that NopProcessor satisfies the Processor interface.
var _ Processor = NopProcessor{}

func (NopProcessor) ProcessStartGame(_ *Context, _ *minecraft.GameData)      {}
func (NopProcessor) ProcessServer(_ *Context, _ *packet.Packet)              {}
func (NopProcessor) ProcessServerEncoded(_ *Context, _ *[]byte)              {}
func (NopProcessor) ProcessClient(_ *Context, _ *packet.Packet)              {}
func (NopProcessor) ProcessClientEncoded(_ *Context, _ *[]byte)              {}
func (NopProcessor) ProcessFlush(_ *Context)                                 {}
func (NopProcessor) ProcessDiscover(_ *Context, target *string)              {}
func (NopProcessor) ProcessDiscoverFallback(_ *Context, target *string)      {}
func (NopProcessor) ProcessPreTransfer(_ *Context, _ *string, _ *string)     {}
func (NopProcessor) ProcessTransferFailure(_ *Context, _ *string, _ *string) {}
func (NopProcessor) ProcessPostTransfer(_ *Context, _ *string, _ *string)    {}
func (NopProcessor) ProcessCache(_ *Context, _ *[]byte)                      {}
func (NopProcessor) ProcessDisconnection(_ *Context, _ *string)              {}
