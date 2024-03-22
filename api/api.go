package api

import (
	"bytes"
	"encoding/binary"
	"github.com/sirupsen/logrus"
	"github.com/spectrum-proxy/spectrum/api/packet"
	"github.com/spectrum-proxy/spectrum/internal"
	"github.com/spectrum-proxy/spectrum/protocol"
	"github.com/spectrum-proxy/spectrum/session"
	"net"
)

type API struct {
	logger   *logrus.Logger
	sessions *session.Registry

	listener net.Listener
	pool     packet.Pool
}

func NewAPI(logger *logrus.Logger, sessions *session.Registry) *API {
	return &API{
		logger:   logger,
		sessions: sessions,
		pool:     packet.NewPool(),
	}
}

func (a *API) Listen(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	a.listener = listener
	return nil
}

func (a *API) Accept() error {
	conn, err := a.listener.Accept()
	if err != nil {
		return err
	}

	go a.handle(conn)
	return nil
}

func (a *API) Close() error {
	if a.listener == nil {
		return nil
	}
	return a.listener.Close()
}

func (a *API) handle(conn net.Conn) {
	defer conn.Close()

	reader := protocol.NewReader(conn)
	for {
		data := reader.ReadPacket()
		packetID := binary.LittleEndian.Uint32(data)
		factory, ok := a.pool[packetID]
		if !ok {
			a.logger.Errorf("unknown packet ID: %v", packetID)
			continue
		}

		buf := internal.BufferPool.Get().(*bytes.Buffer)
		buf.Write(data[4:])

		pk := factory()
		pk.Decode(buf)

		buf.Reset()
		internal.BufferPool.Put(buf)
		switch pk := pk.(type) {
		case *packet.Kick:
			s := a.sessions.GetSessionByUsername(pk.Username)
			if s == nil {
				continue
			}

			s.Disconnect(pk.Reason)
		case *packet.Transfer:
			s := a.sessions.GetSessionByUsername(pk.Username)
			if s == nil {
				continue
			}

			if err := s.Transfer(pk.Addr); err != nil {
				a.logger.Errorf("error transferring session: %v", err)
			}
		}
	}
}
