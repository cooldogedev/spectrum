package server

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"net"
)

type Dialer struct {
	Origin string

	ClientData   login.ClientData
	IdentityData login.IdentityData
}

func (d Dialer) Dial(addr string) (*Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetLinger(0)
		_ = tcpConn.SetReadBuffer(1024 * 1024 * 8)
		_ = tcpConn.SetWriteBuffer(1024 * 1024 * 8)
	}
	c := NewConn(conn, packet.NewServerPool())
	return c, c.login(d.Origin, d.ClientData, d.IdentityData)
}
