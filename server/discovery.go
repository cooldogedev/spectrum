package server

import "github.com/sandertv/gophertunnel/minecraft"

type Discovery interface {
	Discover(conn *minecraft.Conn) (string, error)
	DiscoverFallback(conn *minecraft.Conn) (string, error)
}

type StaticDiscovery struct {
	server         string
	fallbackServer string
}

func NewStaticDiscovery(server string, fallbackServer string) *StaticDiscovery {
	return &StaticDiscovery{
		server:         server,
		fallbackServer: fallbackServer,
	}
}

func (s *StaticDiscovery) Discover(_ *minecraft.Conn) (string, error) {
	return s.server, nil
}

func (s *StaticDiscovery) DiscoverFallback(_ *minecraft.Conn) (string, error) {
	return s.fallbackServer, nil
}
