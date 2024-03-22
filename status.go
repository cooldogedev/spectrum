package spectrum

import "github.com/sandertv/gophertunnel/minecraft"

type StatusProvider struct {
	message string
}

func NewStatusProvider(message string) *StatusProvider {
	return &StatusProvider{message: message}
}

func (s *StatusProvider) ServerStatus(playerCount int, maxPlayers int) minecraft.ServerStatus {
	return minecraft.ServerStatus{
		ServerName:  s.message,
		PlayerCount: playerCount,
		MaxPlayers:  maxPlayers,
	}
}
