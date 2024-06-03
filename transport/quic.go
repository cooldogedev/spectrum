package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/cooldogedev/spectrum/internal"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

type QUIC struct {
	connections map[string]quic.Connection
	logger      internal.Logger
	mu          sync.RWMutex
	timeout     time.Duration
}

func NewQUIC(logger internal.Logger, timeout time.Duration) *QUIC {
	return &QUIC{
		connections: map[string]quic.Connection{},
		logger:      logger,
		timeout:     timeout,
	}
}

func (q *QUIC) Dial(addr string) (io.ReadWriteCloser, error) {
	var conn quic.Connection
	if existing := q.get(addr); existing != nil {
		conn = existing
	} else {
		newConn, err := q.openConnection(addr)
		if err != nil {
			return nil, err
		}
		conn = newConn
	}

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		q.remove(addr)
		return nil, err
	}
	return stream, nil
}

func (q *QUIC) openConnection(addr string) (quic.Connection, error) {
	conn, err := quic.DialAddr(
		context.Background(),
		addr,
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"spectrum"},
		},
		&quic.Config{
			MaxIdleTimeout:                 q.timeout,
			InitialStreamReceiveWindow:     1024 * 1024 * 10,
			MaxStreamReceiveWindow:         1024 * 1024 * 10,
			InitialConnectionReceiveWindow: 1024 * 1024 * 10,
			MaxConnectionReceiveWindow:     1024 * 1024 * 10,
			AllowConnectionWindowIncrease:  func(conn quic.Connection, delta uint64) bool { return false },
			InitialPacketSize:              1350,
			DisablePathMTUDiscovery:        false,
			Tracer:                         qlog.DefaultTracer,
		},
	)
	if err != nil {
		return nil, err
	}

	go func() {
		defer q.remove(addr)

		ctx := conn.Context()
		<-ctx.Done()
		if err := ctx.Err(); err != nil && !errors.Is(context.Canceled, err) {
			q.logger.Errorf("Closed connection to %s: %v", addr, err)
		} else {
			q.logger.Debugf("Closed connection to %s", addr)
		}
	}()
	q.add(addr, conn)
	return conn, nil
}

func (q *QUIC) add(addr string, conn quic.Connection) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.connections[addr] = conn
}

func (q *QUIC) get(addr string) quic.Connection {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.connections[addr]
}

func (q *QUIC) remove(addr string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.connections, addr)
}
