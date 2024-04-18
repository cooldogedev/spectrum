package animation

import (
	"image/color"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Fade struct {
	Colour color.RGBA
	Timing protocol.CameraFadeTimeData
}

func (animation *Fade) Play(conn *minecraft.Conn, _ minecraft.GameData) {
	_ = conn.WritePacket(&packet.CameraInstruction{
		Fade: protocol.Option(protocol.CameraInstructionFade{
			TimeData: protocol.Option(animation.Timing),
			Colour:   protocol.Option(animation.Colour),
		}),
	})
}

func (animation *Fade) Clear(*minecraft.Conn, minecraft.GameData) {}
