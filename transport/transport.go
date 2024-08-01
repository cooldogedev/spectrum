package transport

import (
	"context"
	"io"
)

// Transport defines an interface for establishing server connections.
type Transport interface {
	// Dial initiates a connection to the specified address and returns an
	// io.ReadWriteCloser. The provided context is used for managing
	// timeouts and cancellations.
	Dial(ctx context.Context, addr string) (io.ReadWriteCloser, error)
}
