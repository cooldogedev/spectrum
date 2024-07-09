package transport

import "io"

// Transport defines an interface for establishing server connections.
type Transport interface {
	// Dial connects to the specified address and returns an io.ReadWriteCloser.
	// It returns an error if the connection cannot be established.
	Dial(addr string) (io.ReadWriteCloser, error)
}
