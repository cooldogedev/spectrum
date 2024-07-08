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
	counter int32
	mu      sync.Mutex
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
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		_ = s.conn.CloseWithError(0, "failed to open stream")
		return nil, err
	}

	atomic.AddInt32(&s.counter, 1)
	go func() {
		<-stream.Context().Done()
		s.mu.Lock()
		defer s.mu.Unlock()

		atomic.AddInt32(&s.counter, -1)
		if atomic.LoadInt32(&s.counter) <= 0 {
			_ = s.conn.CloseWithError(0, "no open streams")
		}
	}()
	return stream, nil
}

func (q *QUIC) createSession(addr string) (*session, error) {
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

	s := &session{conn: conn}
	defer q.add(addr, s)

	go func() {
		<-conn.Context().Done()
		s.mu.Lock()
		defer s.mu.Unlock()

		q.remove(addr)
		if err := conn.Context().Err(); err != nil && !errors.Is(err, context.Canceled) {
			q.logger.Error("closed connection", "addr", addr, "err", err)
		} else {
			q.logger.Debug("closed connection", "addr", addr)
		}
	}()
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
