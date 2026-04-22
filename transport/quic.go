package transport

import (
	"context"
	"crypto/tls"
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
	connections map[string]*quic.Conn
	logger      *slog.Logger
	mu          sync.Mutex
}

// NewQUIC creates a new QUIC transport instance.
func NewQUIC(logger *slog.Logger) *QUIC {
	return &QUIC{
		connections: make(map[string]*quic.Conn),
		logger:      logger,
	}
}

// Dial ...
func (q *QUIC) Dial(ctx context.Context, addr string) (io.ReadWriteCloser, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	conn, ok := q.connections[addr]
	if !ok {
		c, err := quic.DialAddr(
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

		conn = c
		q.connections[addr] = conn
		q.logger.Debug("established connection", "addr", addr)
		go func(conn *quic.Conn, addr string) {
			<-conn.Context().Done()
			q.mu.Lock()
			if found, ok := q.connections[addr]; ok && found == conn {
				delete(q.connections, addr)
			}
			q.mu.Unlock()
			q.logger.Debug("closed connection", "addr", addr, "cause", context.Cause(conn.Context()))
		}(conn, addr)
	}
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		delete(q.connections, addr)
		return nil, err
	}
	return stream, nil
}
