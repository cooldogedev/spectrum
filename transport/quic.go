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
	timer   *time.Timer
}

func (s *session) openStream() (quic.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.conn.Context().Done():
		return nil, net.ErrClosed
	default:
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		stream, err := s.conn.OpenStreamSync(ctx)
		if err != nil {
			_ = s.conn.CloseWithError(0, "failed to open stream")
			return nil, err
		}

		if s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}

		s.counter++
		go func() {
			<-stream.Context().Done()
			s.closeStream()
		}()
		return stream, nil
	}
}

func (s *session) closeStream() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter--
	if s.counter > 0 {
		return
	}

	if s.timer != nil {
		s.timer.Reset(time.Second * 10)
		return
	}
	s.timer = time.AfterFunc(time.Second*10, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.timer = nil
		if s.counter <= 0 {
			_ = s.conn.CloseWithError(0, "no open streams")
		}
	})
}

// QUIC implements the Transport interface to establish connections to servers using the QUIC protocol.
// It maintains a single connection per server, optimizing dialing times and reducing connection overhead.
// By leveraging streams for individual server connections, it enhances overall performance and
// resource utilization.
type QUIC struct {
	sessions map[string]*session
	logger   *slog.Logger
	mu       sync.Mutex
}

// NewQUIC creates a new QUIC transport instance.
func NewQUIC(logger *slog.Logger) *QUIC {
	return &QUIC{
		sessions: make(map[string]*session),
		logger:   logger,
	}
}

// Dial ...
func (q *QUIC) Dial(addr string) (io.ReadWriteCloser, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if s, ok := q.sessions[addr]; ok {
		stream, err := s.openStream()
		if err == nil {
			return stream, nil
		}

		if !errors.Is(err, net.ErrClosed) {
			return nil, err
		}
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
		q.logger.Debug("failed to dial address", "addr", addr, "err", err)
		return nil, err
	}

	s := &session{conn: conn}
	q.sessions[addr] = s
	q.logger.Debug("established connection", "addr", addr)
	go func() {
		<-conn.Context().Done()
		q.mu.Lock()
		delete(q.sessions, addr)
		q.mu.Unlock()
		if err := conn.Context().Err(); err != nil && !errors.Is(err, context.Canceled) {
			q.logger.Error("closed connection", "addr", addr, "err", err)
		} else {
			q.logger.Debug("closed connection", "addr", addr)
		}
	}()
	return s.openStream()
}
