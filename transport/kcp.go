package transport

import (
	"context"
	"github.com/xtaci/kcp-go"
	"io"
	"log/slog"
)

// KCP implements the Transport interface to establish connections to servers using the KCP protocol.
type KCP struct{}

// NewKCP creates a new KCP transport instance.
func NewKCP(logger *slog.Logger) *KCP {
	return &KCP{}
}

// Dial ...
func (k *KCP) Dial(_ context.Context, addr string) (io.ReadWriteCloser, error) {
	conn, err := kcp.DialWithOptions(addr, nil, 10, 3)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
