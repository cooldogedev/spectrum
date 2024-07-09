package transport

import (
	"io"
	"net"
)

// TCP implements the Transport interface to establish connections to servers using the TCP protocol.
type TCP struct{}

// NewTCP creates a new TCP transport instance.
func NewTCP() *TCP {
	return &TCP{}
}

// Dial ...
func (t *TCP) Dial(addr string) (io.ReadWriteCloser, error) {
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
	return conn, nil
}
