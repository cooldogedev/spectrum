package transport

import (
	"io"

	"github.com/sandertv/gophertunnel/minecraft"
)

type RakNet struct{}

func NewRakNet() *RakNet {
	return &RakNet{}
}

func (r *RakNet) Dial(addr string) (io.ReadWriteCloser, error) {
	conn, err := minecraft.Dial("raknet", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
