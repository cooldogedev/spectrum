package api

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/protocol"
	"github.com/cooldogedev/spectrum/session"
	"github.com/sirupsen/logrus"
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

	closed := make(chan struct{})
	reader := protocol.NewReader(conn)
	go func() {
		for {
			select {
			case <-closed:
				return
			default:
				if err := reader.Read(); err != nil {
					close(closed)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-closed:
			return
		default:
			pk, err := a.decodePacket(reader.ReadPacket())
			if err != nil {
				a.logger.Errorf("Failed to decode packet: %v", err)
				close(closed)
				return
			}
			a.handlePacket(pk)
		}
	}
}

func (a *API) decodePacket(payload []byte) (pk packet.Packet, err error) {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)

		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet: %v", r)
		}
	}()

	packetID := binary.LittleEndian.Uint32(payload[:4])
	factory, ok := a.pool[packetID]
	if !ok {
		return nil, fmt.Errorf("unknown packet ID: %v", packetID)
	}

	buf.Write(payload[4:])
	pk = factory()
	pk.Decode(buf)
	return
}

func (a *API) handlePacket(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.Kick:
		s := a.sessions.GetSessionByUsername(pk.Username)
		if s != nil {
			s.Disconnect(pk.Reason)
		}
	case *packet.Transfer:
		s := a.sessions.GetSessionByUsername(pk.Username)
		if s != nil {
			_ = s.Transfer(pk.Addr)
		}
	}
}
