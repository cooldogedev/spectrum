package util

import "github.com/sandertv/gophertunnel/minecraft"

type StatusProvider struct {
	serverName    string
	serverSubName string
}

func NewStatusProvider(serverName string, serverSubName string) *StatusProvider {
	return &StatusProvider{serverName: serverName, serverSubName: serverSubName}
}

func (s *StatusProvider) ServerStatus(playerCount int, maxPlayers int) minecraft.ServerStatus {
	return minecraft.ServerStatus{
		ServerName:    s.serverName,
		ServerSubName: s.serverSubName,
		PlayerCount:   playerCount,
		MaxPlayers:    maxPlayers,
	}
}
