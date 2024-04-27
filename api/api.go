package api

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/protocol"
	"github.com/cooldogedev/spectrum/session"
	"github.com/sirupsen/logrus"
)

type API struct {
	authentication Authentication
	sessions       *session.Registry
	logger         *logrus.Logger

	listener net.Listener
	pool     packet.Pool
}

func NewAPI(sessions *session.Registry, logger *logrus.Logger, authentication Authentication) *API {
	return &API{
		authentication: authentication,
		sessions:       sessions,
		logger:         logger,
		pool:           packet.NewPool(),
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

	if conn, ok := conn.(*net.TCPConn); ok {
		_ = conn.SetLinger(0)
		_ = conn.SetNoDelay(true)
		_ = conn.SetReadBuffer(1024 * 1024 * 8)
		_ = conn.SetWriteBuffer(1024 * 1024 * 8)
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
	writer := protocol.NewWriter(conn)
	connectionRequestBytes, err := reader.ReadPacket()
	if err != nil {
		a.logger.Errorf("Failed to read connection request: %v", err)
		return
	}

	connectionRequestPacket, err := a.decodePacket(connectionRequestBytes)
	if err != nil {
		a.logger.Errorf("Failed to decode connection request: %v", err)
		_ = writer.Write(a.encodePacket(&packet.ConnectionResponse{
			Response: packet.ResponseFail,
		}))
		return
	}

	connectionRequest, ok := connectionRequestPacket.(*packet.ConnectionRequest)
	if !ok {
		a.logger.Errorf("Expected connection request, received: %d", connectionRequest.ID())
		_ = writer.Write(a.encodePacket(&packet.ConnectionResponse{
			Response: packet.ResponseFail,
		}))
		return
	}

	if a.authentication != nil && !a.authentication.Authenticate(connectionRequest.Token) {
		a.logger.Debugf("Closed unauthenticated connection from %s, token: %s", conn.RemoteAddr().String(), connectionRequest.Token)
		_ = writer.Write(a.encodePacket(&packet.ConnectionResponse{
			Response: packet.ResponseUnauthorized,
		}))
		return
	}

	_ = writer.Write(a.encodePacket(&packet.ConnectionResponse{
		Response: packet.ResponseSuccess,
	}))
	for {
		payload, err := reader.ReadPacket()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				a.logger.Errorf("Failed to read packet: %v", err)
			}
			return
		}

		pk, err := a.decodePacket(payload)
		if err != nil {
			a.logger.Errorf("Failed to decode packet: %v", err)
			return
		}
		a.handlePacket(pk)
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

func (a *API) encodePacket(pk packet.Packet) []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	_ = binary.Write(buf, binary.LittleEndian, pk.ID())
	pk.Encode(buf)
	return buf.Bytes()
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
