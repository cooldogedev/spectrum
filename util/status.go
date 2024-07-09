package util

import "github.com/sandertv/gophertunnel/minecraft"

// StatusProvider implements the minecraft.ServerStatusProvider interface.
// It provides server status information including the server name, sub-name, player count, and max players.
type StatusProvider struct {
	serverName    string
	serverSubName string
}

// NewStatusProvider creates a new StatusProvider with the given server name and sub-name.
func NewStatusProvider(serverName string, serverSubName string) *StatusProvider {
	return &StatusProvider{serverName: serverName, serverSubName: serverSubName}
}

// ServerStatus ...
func (s *StatusProvider) ServerStatus(playerCount int, maxPlayers int) minecraft.ServerStatus {
	return minecraft.ServerStatus{
		ServerName:    s.serverName,
		ServerSubName: s.serverSubName,
		PlayerCount:   playerCount,
		MaxPlayers:    maxPlayers,
	}
}
