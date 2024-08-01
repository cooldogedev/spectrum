package transport

import (
	"context"
	"io"

	"github.com/sandertv/gophertunnel/minecraft"
)

// RakNet implements the Transport interface to establish connections to servers using the RakNet protocol.
type RakNet struct{}

// NewRakNet creates a new RakNet transport instance.
func NewRakNet() *RakNet {
	return &RakNet{}
}

// Dial ...
func (r *RakNet) Dial(ctx context.Context, addr string) (io.ReadWriteCloser, error) {
	conn, err := minecraft.DialContext(ctx, "raknet", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
