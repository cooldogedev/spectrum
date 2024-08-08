package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

// QUIC implements the Transport interface to establish connections to servers using the QUIC protocol.
// It maintains a single connection per server, optimizing dialing times and reducing connection overhead.
// By leveraging streams for individual server connections, it enhances overall performance and
// resource utilization.
type QUIC struct {
	connections map[string]quic.Connection
	logger      *slog.Logger
	mu          sync.Mutex
}

// NewQUIC creates a new QUIC transport instance.
func NewQUIC(logger *slog.Logger) *QUIC {
	return &QUIC{
		connections: make(map[string]quic.Connection),
		logger:      logger,
	}
}

// Dial ...
func (q *QUIC) Dial(ctx context.Context, addr string) (io.ReadWriteCloser, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if conn, ok := q.connections[addr]; ok {
		stream, err := conn.OpenStreamSync(ctx)
		if err != nil {
			_ = conn.CloseWithError(0, "failed to open stream")
			return nil, err
		}
		return stream, nil
	}

	conn, err := quic.DialAddr(
		ctx,
		addr,
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"spectrum"},
		},
		&quic.Config{
			MaxIdleTimeout:                 time.Second * 10,
			InitialStreamReceiveWindow:     1024 * 1024 * 10,
			InitialConnectionReceiveWindow: 1024 * 1024 * 10,
			KeepAlivePeriod:                0,
			InitialPacketSize:              1350,
			Tracer:                         qlog.DefaultConnectionTracer,
		},
	)
	if err != nil {
		return nil, err
	}

	q.connections[addr] = conn
	q.logger.Debug("established connection", "addr", addr)
	go func() {
		<-conn.Context().Done()
		q.mu.Lock()
		delete(q.connections, addr)
		q.mu.Unlock()
		if err := conn.Context().Err(); err != nil && !errors.Is(err, context.Canceled) {
			q.logger.Error("closed connection", "addr", addr, "err", err)
		} else {
			q.logger.Debug("closed connection", "addr", addr)
		}
	}()
	return conn.OpenStreamSync(ctx)
}
