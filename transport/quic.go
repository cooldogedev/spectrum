package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

type session struct {
	conn    quic.Connection
	counter int32
	mu      sync.Mutex
	ch      chan struct{}
	closed  atomic.Bool
}

func createSession(conn quic.Connection) *session {
	return &session{
		conn: conn,
		ch:   make(chan struct{}),
	}
}

func (s *session) openStream() (quic.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.ch:
		return nil, net.ErrClosed
	default:
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		stream, err := s.conn.OpenStreamSync(ctx)
		if err != nil {
			_ = s.close()
			return nil, err
		}
		s.counter++
		go s.monitorStream(stream.Context())
		return stream, nil
	}
}

func (s *session) monitorStream(ctx context.Context) {
	select {
	case <-s.ch:
	case <-ctx.Done():
		s.closeStream()
	}
}

func (s *session) closeStream() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter--
	if s.counter <= 0 {
		_ = s.conn.CloseWithError(0, "no open streams")
	}
}

func (s *session) close() error {
	if s.closed.Load() {
		return net.ErrClosed
	}

	select {
	case <-s.ch:
		return net.ErrClosed
	default:
		close(s.ch)
		s.closed.Store(true)
		_ = s.conn.CloseWithError(0, "")
		return nil
	}
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
	if s := q.get(addr); s != nil && !s.closed.Load() {
		s, err := s.openStream()
		if err == nil {
			return s, nil
		}

		if errors.Is(err, net.ErrClosed) {
			return q.Dial(addr)
		}
		q.logger.Error("error opening stream", "addr", addr, "err", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

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
			MaxStreamReceiveWindow:         1024 * 1024 * 10,
			InitialConnectionReceiveWindow: 1024 * 1024 * 10,
			MaxConnectionReceiveWindow:     1024 * 1024 * 10,
			KeepAlivePeriod:                time.Second * 5,
			InitialPacketSize:              1350,
			Tracer:                         qlog.DefaultConnectionTracer,
		},
	)
	if err != nil {
		return nil, err
	}
	go q.monitorConnection(addr, conn.Context())
	s := createSession(conn)
	q.add(addr, s)
	return s.openStream()
}

func (q *QUIC) monitorConnection(addr string, ctx context.Context) {
	<-ctx.Done()
	q.remove(addr)
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		q.logger.Error("closed connection", "addr", addr, "err", err)
	} else {
		q.logger.Debug("closed connection", "addr", addr)
	}
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
