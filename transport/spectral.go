package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/cooldogedev/spectral"
)

// Spectral implements the Transport interface to establish connections to servers using Spectral.
// It maintains a single connection per server, optimizing dialing times and reducing connection overhead.
// By leveraging streams for individual server connections, it enhances overall performance and
// resource utilization.
type Spectral struct {
	connections map[string]spectral.Connection
	logger      *slog.Logger
	mu          sync.Mutex
}

// NewSpectral creates a new Spectral transport instance.
func NewSpectral(logger *slog.Logger) *Spectral {
	return &Spectral{
		connections: make(map[string]spectral.Connection),
		logger:      logger,
	}
}

// Dial ...
func (s *Spectral) Dial(ctx context.Context, addr string) (io.ReadWriteCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, ok := s.connections[addr]
	if !ok {
		c, err := spectral.Dial(ctx, addr)
		if err != nil {
			return nil, err
		}
		conn = c
		s.connections[addr] = conn
		s.logger.Debug("established connection", "addr", addr)
		go func() {
			<-conn.Context().Done()
			s.mu.Lock()
			delete(s.connections, addr)
			s.mu.Unlock()
			if err := conn.Context().Err(); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("closed connection", "addr", addr, "err", err)
			} else {
				s.logger.Debug("closed connection", "addr", addr)
			}
		}()
	}
	stream, err := conn.OpenStream(ctx)
	if err != nil {
		_ = conn.CloseWithError(0, "failed to open stream")
		return nil, err
	}
	return stream, nil
}
