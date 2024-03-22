package animation

import (
	"github.com/sandertv/gophertunnel/minecraft"
)

type Animation interface {
	Play(conn *minecraft.Conn, serverGameData minecraft.GameData)
	Clear(conn *minecraft.Conn, serverGameData minecraft.GameData)
}
