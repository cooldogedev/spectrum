package animation

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Dimension displays the dimension change screen to the player.
type Dimension struct{}

// Play ...
func (animation *Dimension) Play(conn *minecraft.Conn, serverGameData minecraft.GameData) {
	var dimension int32
	if conn.GameData().Dimension == packet.DimensionNether {
		dimension = packet.DimensionEnd
	} else {
		dimension = packet.DimensionNether
	}
	sendDimension(conn, serverGameData, dimension, false)
}

// Clear ...
func (animation *Dimension) Clear(conn *minecraft.Conn, serverGameData minecraft.GameData) {
	_ = conn.WritePacket(&packet.PlayStatus{Status: packet.PlayStatusPlayerSpawn})
	sendDimension(conn, serverGameData, packet.DimensionOverworld, true)
}

// sendDimension updates the player's dimension and optionally force-spawns them if playStatus is enabled.
func sendDimension(conn *minecraft.Conn, serverGameData minecraft.GameData, dimension int32, playStatus bool) {
	_ = conn.WritePacket(&packet.ChangeDimension{Dimension: dimension, Position: serverGameData.PlayerPosition})
	_ = conn.WritePacket(&packet.StopSound{StopAll: true})
	_ = conn.WritePacket(&packet.PlayerAction{ActionType: protocol.PlayerActionDimensionChangeDone})
	if playStatus {
		_ = conn.WritePacket(&packet.PlayStatus{
			Status: packet.PlayStatusPlayerSpawn,
		})
	}
}
