package animation

import (
	"sync/atomic"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Animation is an interface used by sessions to manage visual animations during server transfers.
type Animation interface {
	// Play starts the animation on the given connection using the new server's GameData.
	Play(conn *minecraft.Conn, serverGameData minecraft.GameData)
	// Clear stops the animation previously started on the connection using the new server's GameData.
	Clear(conn *minecraft.Conn, serverGameData minecraft.GameData)
}

// NopAnimation is a no-operation implementation of the Animation interface.
type NopAnimation struct{}

// Ensure that NopAnimation satisfies the Animation interface.
var _ Animation = NopAnimation{}

func (NopAnimation) Play(_ *minecraft.Conn, _ minecraft.GameData)  {}
func (NopAnimation) Clear(_ *minecraft.Conn, _ minecraft.GameData) {}

type cameraAnimation struct {
	synced atomic.Bool
}

func (animation *cameraAnimation) Sync(conn *minecraft.Conn) {
	if animation.synced.CompareAndSwap(false, true) {
		_ = conn.WritePacket(&packet.CameraPresets{
			Presets: []protocol.CameraPreset{
				{
					Name: "minecraft:free",
				},
			},
		})
	}
}
