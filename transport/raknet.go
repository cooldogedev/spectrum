package transport

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"io"
)

type RakNet struct{}

func NewRakNet() *RakNet {
	return &RakNet{}
}

func (t *RakNet) Dial(addr string) (io.ReadWriteCloser, error) {
	conn, err := minecraft.Dial("raknet", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
