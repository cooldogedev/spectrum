package animation

import (
	"image/color"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Fade represents a static fade animation that gradually changes the screen to a specified color.
// The animation's timing values for fade-in, fade-out, and wait durations must be carefully chosen,
// as the animation does not dynamically adjust to changes in transfer time.
type Fade struct {
	NopAnimation

	Colour color.RGBA
	Timing protocol.CameraFadeTimeData
}

// Play ...
func (animation *Fade) Play(conn *minecraft.Conn, _ minecraft.GameData) {
	_ = conn.WritePacket(&packet.CameraInstruction{
		Fade: protocol.Option(protocol.CameraInstructionFade{
			TimeData: protocol.Option(animation.Timing),
			Colour:   protocol.Option(animation.Colour),
		}),
	})
}
