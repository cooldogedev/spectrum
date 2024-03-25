package server

import (
	"context"
	"crypto/tls"
	"github.com/quic-go/quic-go"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"time"
)

type Dialer struct {
	Origin       string
	ClientData   login.ClientData
	IdentityData login.IdentityData
}

func (d Dialer) Dial(addr string) (*Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"spectrum"},
	}, nil)
	if err != nil {
		return nil, err
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	c := NewConn(conn, stream, packet.NewServerPool())
	return c, c.login(d.Origin, d.ClientData, d.IdentityData)
}
