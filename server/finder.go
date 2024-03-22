package server

import "github.com/sandertv/gophertunnel/minecraft"

type Finder interface {
	Find(conn *minecraft.Conn) (string, error)
}

type BasicFinder struct {
	server string
}

func (s *BasicFinder) Find(*minecraft.Conn) (string, error) {
	return s.server, nil
}

func NewBasicFinder(server string) *BasicFinder {
	return &BasicFinder{
		server: server,
	}
}
