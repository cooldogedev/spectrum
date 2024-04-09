package server

import (
	"net"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Dialer struct {
	Origin       string
	Token 		 string
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
	return c, c.connect(d.Origin, d.Token, d.ClientData, d.IdentityData)
}
