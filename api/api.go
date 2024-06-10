package api

import (
	"errors"
	"io"
	"net"

	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/session"
)

type handler = func(packet.Packet) bool

type API struct {
	authentication Authentication
	handlers       map[uint32]handler
	registry       *session.Registry

	closed   chan struct{}
	listener net.Listener
	logger   internal.Logger
}

func NewAPI(registry *session.Registry, logger internal.Logger, authentication Authentication) *API {
	a := &API{
		authentication: authentication,
		handlers:       map[uint32]handler{},
		registry:       registry,

		closed: make(chan struct{}),
		logger: logger,
	}
	a.RegisterHandler(packet.IDKick, func(pk packet.Packet) bool {
		username := pk.(*packet.Kick).Username
		reason := pk.(*packet.Kick).Username
		if s := a.registry.GetSessionByUsername(username); s != nil {
			s.Disconnect(reason)
		} else {
			a.logger.Debugf("Tried to disconnect an unknown player %s for %s", username, reason)
		}
		return true
	})
	a.RegisterHandler(packet.IDTransfer, func(pk packet.Packet) bool {
		username := pk.(*packet.Transfer).Username
		addr := pk.(*packet.Transfer).Addr
		if s := a.registry.GetSessionByUsername(username); s != nil {
			if err := s.Transfer(addr); err != nil {
				a.logger.Errorf("Failed to transfer player %s to %s: %v", username, addr, err)
			}
		} else {
			a.logger.Debugf("Tried to transfer an unknown player %s to %s", username, addr)
		}
		return true
	})
	return a
}

func (a *API) Listen(addr string) (err error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	a.listener = listener
	return
}

func (a *API) Accept() (err error) {
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
	return
}

func (a *API) RegisterHandler(packet uint32, h handler) {
	a.handlers[packet] = h
}

func (a *API) Close() (err error) {
	select {
	case <-a.closed:
		return errors.New("already closed")
	default:
		close(a.closed)
		if a.listener != nil {
			_ = a.listener.Close()
		}
		return
	}
}

func (a *API) handle(conn net.Conn) {
	identifier := conn.RemoteAddr().String()
	a.logger.Debugf("Accepted connection from %s", identifier)

	c := NewClient(conn, packet.NewPool())
	defer func() {
		_ = c.Close()
		a.logger.Infof("Disconnected connection from %s", identifier)
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
		a.logger.Debugf("Closed unauthenticated connection from %s, token: %s", identifier, connectionRequest.Token)
		return
	}

	if err := c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseSuccess}); err != nil {
		a.logger.Errorf("Failed to write connection response: %v", err)
		return
	}

	a.logger.Infof("Successfully authorized connection from %s", identifier)
	for {
		select {
		case <-a.closed:
			return
		default:
			pk, err := c.ReadPacket()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					a.logger.Errorf("Failed to read packet to %s: %v", identifier, err)
				}
				return
			}

			if h, ok := a.handlers[pk.ID()]; ok {
				if !h(pk) {
					a.logger.Errorf("Failed to handle packet from %s: %d", identifier, pk.ID())
					return
				}
			} else {
				a.logger.Errorf("Received an unhandled packet from %s: %d", identifier, pk.ID())
			}
		}
	}
}
