package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

type session struct {
	conn    quic.Connection
	counter int32
	mu      sync.Mutex
}

func createSession() *session {
	s := &session{}
	s.mu.Lock()
	return s
}

func (s *session) openStream() (quic.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil, net.ErrClosed
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		_ = s.conn.CloseWithError(0, "failed to open stream")
		return nil, err
	}

	s.counter++
	go func() {
		<-stream.Context().Done()
		s.mu.Lock()
		defer s.mu.Unlock()

		s.counter--
		if s.counter <= 0 {
			_ = s.conn.CloseWithError(0, "no open streams")
		}
	}()
	return stream, nil
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
		stream, err := s.openStream()
		if errors.Is(err, net.ErrClosed) {
			q.logger.Debug("tried to open stream on a closed connection", "addr", addr)
			return q.Dial(addr)
		}
		return stream, err
	}

	q.mu.Lock()
	s := createSession()
	q.sessions[addr] = s
	q.mu.Unlock()
	q.logger.Debug("created connection", "addr", addr)

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
		s.mu.Unlock()
		q.remove(addr)
		q.logger.Debug("failed to dial address", "addr", addr, "err", err)
		return nil, err
	}

	s.conn = conn
	s.mu.Unlock()
	q.logger.Debug("successfully dialed address", "addr", addr)
	go func() {
		<-conn.Context().Done()
		q.remove(addr)
		if err := conn.Context().Err(); err != nil && !errors.Is(err, context.Canceled) {
			q.logger.Error("closed connection", "addr", addr, "err", err)
		} else {
			q.logger.Debug("closed connection", "addr", addr)
		}
	}()
	return s.openStream()
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
