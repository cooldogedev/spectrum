package animation

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Animation interface {
	Play(conn *minecraft.Conn, serverGameData minecraft.GameData)
	Clear(conn *minecraft.Conn, serverGameData minecraft.GameData)
}

type CameraAnimation struct {
	synced bool
}

func (animation *CameraAnimation) Sync(conn *minecraft.Conn) {
	if !animation.synced {
		animation.synced = true
		_ = conn.WritePacket(&packet.CameraPresets{
			Presets: []protocol.CameraPreset{
				{
					Name: "minecraft:free",
				},
			},
		})
	}
}
