package server

import "github.com/sandertv/gophertunnel/minecraft"

type Discovery interface {
	Discover(conn *minecraft.Conn) (string, error)
}

type StaticDiscovery struct {
	server string
}

func (s *StaticDiscovery) Discover(*minecraft.Conn) (string, error) {
	return s.server, nil
}

func NewStaticDiscovery(server string) *StaticDiscovery {
	return &StaticDiscovery{
		server: server,
	}
}
