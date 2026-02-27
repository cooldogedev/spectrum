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
	tlsConfig   *tls.Config
	logger      *slog.Logger
	mu          sync.Mutex
}

// NewQUIC creates a new QUIC transport instance.
func NewQUIC(logger *slog.Logger) *QUIC {
	return &QUIC{
		connections: make(map[string]*quic.Conn),
		tlsConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			NextProtos: []string{"spectrum"},
		},
		logger: logger,
	}
}

// NewQUICInsecure creates a QUIC transport that skips certificate verification.
// It is intended for development and testing only.
func NewQUICInsecure(logger *slog.Logger) *QUIC {
	return &QUIC{
		connections: make(map[string]*quic.Conn),
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
			NextProtos:         []string{"spectrum"},
		},
		logger: logger,
	}
}

// Dial ...
func (q *QUIC) Dial(ctx context.Context, addr string) (io.ReadWriteCloser, error) {
	conn, err := q.getOrDial(ctx, addr)
	if err != nil {
		return nil, err
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		if conn.Context().Err() != nil {
			q.mu.Lock()
			if found, ok := q.connections[addr]; ok && found == conn {
				delete(q.connections, addr)
			}
			q.mu.Unlock()
		}
		return nil, err
	}
	return stream, nil
}

func (q *QUIC) getOrDial(ctx context.Context, addr string) (*quic.Conn, error) {
	q.mu.Lock()
	conn, ok := q.connections[addr]
	q.mu.Unlock()
	if !ok {
		tlsConfig := q.tlsConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{MinVersion: tls.VersionTLS13, NextProtos: []string{"spectrum"}}
		}

		c, err := quic.DialAddr(
			ctx,
			addr,
			tlsConfig.Clone(),
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

		q.mu.Lock()
		if existing, exists := q.connections[addr]; exists {
			q.mu.Unlock()
			_ = c.CloseWithError(0, "duplicate connection")
			return existing, nil
		}

		conn = c
		q.connections[addr] = conn
		q.mu.Unlock()
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
	return conn, nil
}
