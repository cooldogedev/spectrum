package api

import (
	"errors"
	"io"
	"log/slog"
	"net"

	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/session"
)

type handler = func(packet.Packet) bool

type API struct {
	authentication Authentication
	handlers       map[uint32]handler
	registry       *session.Registry

	closed   chan struct{}
	listener net.Listener
	logger   *slog.Logger
}

func NewAPI(registry *session.Registry, logger *slog.Logger, authentication Authentication) *API {
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
			a.logger.Debug("tried to disconnect an unknown player", "username", username, "reason", reason)
		}
		return true
	})
	a.RegisterHandler(packet.IDTransfer, func(pk packet.Packet) bool {
		username := pk.(*packet.Transfer).Username
		addr := pk.(*packet.Transfer).Addr
		if s := a.registry.GetSessionByUsername(username); s != nil {
			if err := s.Transfer(addr); err != nil {
				a.logger.Error("failed to transfer player", "username", username, "addr", addr, "err", err)
			}
		} else {
			a.logger.Debug("tried to transfer an unknown player", "username", username, "addr", addr)
		}
		return true
	})
	return a
}

func (a *API) Listen(addr string) (err error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		a.logger.Error("failed to start listening", "addr", addr, "err", err)
		return err
	}
	a.listener = listener
	a.logger.Info("started listening on", "addr", addr)
	return
}

func (a *API) Accept() (err error) {
	conn, err := a.listener.Accept()
	if err != nil {
		a.logger.Error("sailed to accept connection", "err", err)
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
	a.logger.Debug("accepted connection", "addr", identifier)

	c := NewClient(conn, packet.NewPool())
	defer func() {
		_ = c.Close()
		a.logger.Info("disconnected connection", "addr", identifier)
	}()

	connectionRequestPacket, err := c.ReadPacket()
	if err != nil {
		_ = c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseFail})
		a.logger.Error("failed to read connection request", "addr", identifier, "err", err)
		return
	}

	connectionRequest, ok := connectionRequestPacket.(*packet.ConnectionRequest)
	if !ok {
		_ = c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseFail})
		a.logger.Error("expected connection request", "addr", identifier, "pid", connectionRequestPacket.ID())
		return
	}

	if a.authentication != nil && !a.authentication.Authenticate(connectionRequest.Token) {
		_ = c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseUnauthorized})
		a.logger.Error("unauthorized connection", "addr", identifier, "token", connectionRequest.Token)
		return
	}

	if err := c.WritePacket(&packet.ConnectionResponse{Response: packet.ResponseSuccess}); err != nil {
		a.logger.Error("failed to write connection response", "err", err)
		return
	}

	a.logger.Info("successfully authorized connection", "addr", identifier)
	for {
		select {
		case <-a.closed:
			return
		default:
			pk, err := c.ReadPacket()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					a.logger.Error("failed to read packet", "addr", identifier, "err", err)
				}
				return
			}

			if h, ok := a.handlers[pk.ID()]; ok {
				if !h(pk) {
					a.logger.Error("failed to handle packet", "addr", identifier, "pid", pk.ID())
					return
				}
			} else {
				a.logger.Error("received an unhandled packet", "addr", identifier, "pid", pk.ID())
			}
		}
	}
}
