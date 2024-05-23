package api

import (
	"errors"
	"io"
	"net"

	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/session"
)

type API struct {
	authentication Authentication
	sessions       *session.Registry
	listener       net.Listener
	logger         internal.Logger
}

func NewAPI(sessions *session.Registry, logger internal.Logger, authentication Authentication) *API {
	return &API{
		authentication: authentication,
		sessions:       sessions,
		logger:         logger,
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
	a.logger.Infof("Accepted connection from %s", conn.RemoteAddr().String())
	return nil
}

func (a *API) Close() error {
	if a.listener == nil {
		return nil
	}
	return a.listener.Close()
}

func (a *API) handle(conn net.Conn) {
	c := NewClient(conn, packet.NewPool())
	defer func() {
		_ = c.Close()
		a.logger.Infof("Disconnected connection from %s", conn.RemoteAddr().String())
	}()

	connectionRequestPacket, err := c.ReadPacket()
	if err != nil {
		_ = c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseFail})
		a.logger.Errorf("Failed to read connection request: %v", err)
		return
	}

	connectionRequest, ok := connectionRequestPacket.(*packet.ConnectionRequest)
	if !ok {
		_ = c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseFail})
		a.logger.Errorf("Expected connection request, got: %d", connectionRequest.ID())
		return
	}

	if a.authentication != nil && !a.authentication.Authenticate(connectionRequest.Token) {
		_ = c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseUnauthorized})
		a.logger.Debugf("Closed unauthenticated connection from %s, token: %s", conn.RemoteAddr().String(), connectionRequest.Token)
		return
	}

	_ = c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseSuccess})
	a.logger.Infof("Successfully authorized connection from %s", conn.RemoteAddr().String())
	for {
		pk, err := c.ReadPacket()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				a.logger.Errorf("Failed to read packet: %v", err)
			}
			return
		}

		switch pk := pk.(type) {
		case *packet.Kick:
			s := a.sessions.GetSessionByUsername(pk.Username)
			if s == nil {
				a.logger.Debugf("Tried to disconnect an unknown player %s", pk.Username)
				return
			}
			s.Disconnect(pk.Reason)
		case *packet.Transfer:
			s := a.sessions.GetSessionByUsername(pk.Username)
			if s == nil {
				a.logger.Debugf("Tried to transfer an unknown player %s", pk.Username)
				return
			}

			if err := s.Transfer(pk.Addr); err != nil {
				a.logger.Errorf("Failed to transfer player %s: %v", pk.Username, err)
			}
		}
	}
}
