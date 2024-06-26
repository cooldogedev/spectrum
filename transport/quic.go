package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

type session struct {
	conn    quic.Connection
	counter atomic.Int64
}

type QUIC struct {
	sessions map[string]*session
	logger   *slog.Logger
	mu       sync.RWMutex
}

func NewQUIC(logger *slog.Logger) *QUIC {
	return &QUIC{
		sessions: make(map[string]*session),
		logger:   logger,
	}
}

func (q *QUIC) Dial(addr string) (io.ReadWriteCloser, error) {
	if s := q.get(addr); s != nil {
		return q.openStream(s)
	}

	s, err := q.createSession(addr)
	if err != nil {
		return nil, err
	}
	return q.openStream(s)
}

func (q *QUIC) openStream(s *session) (io.ReadWriteCloser, error) {
	stream, err := s.conn.OpenStreamSync(context.Background())
	if err != nil {
		return nil, err
	}

	s.counter.Add(1)
	go func() {
		<-stream.Context().Done()
		s.counter.Add(-1)
		if s.counter.Load() <= 0 {
			_ = s.conn.CloseWithError(0, "")
		}
	}()
	return stream, nil
}

func (q *QUIC) createSession(addr string) (*session, error) {
	conn, err := quic.DialAddr(
		context.Background(),
		addr,
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"spectrum"},
		},
		&quic.Config{
			MaxIdleTimeout:                 time.Second * 2,
			InitialStreamReceiveWindow:     1024 * 1024 * 10,
			MaxStreamReceiveWindow:         1024 * 1024 * 10,
			InitialConnectionReceiveWindow: 1024 * 1024 * 10,
			MaxConnectionReceiveWindow:     1024 * 1024 * 10,
			AllowConnectionWindowIncrease:  func(conn quic.Connection, delta uint64) bool { return false },
			InitialPacketSize:              1350,
			DisablePathMTUDiscovery:        false,
			Tracer:                         qlog.DefaultConnectionTracer,
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
			q.logger.Error("closed connection", "addr", addr, "err", err)
		} else {
			q.logger.Debug("closed connection", "addr", addr)
		}
	}()

	s := &session{conn: conn}
	q.add(addr, s)
	return s, nil
}

func (q *QUIC) add(addr string, s *session) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sessions[addr] = s
}

func (q *QUIC) get(addr string) *session {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.sessions[addr]
}

func (q *QUIC) remove(addr string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.sessions, addr)
}
