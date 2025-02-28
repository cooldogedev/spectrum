package spectrum

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	tr "github.com/cooldogedev/spectrum/transport"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
)

// Spectrum represents a proxy server managing server discovery,
// network transport, and connections through an underlying minecraft.Listener.
type Spectrum struct {
	discovery server.Discovery
	transport tr.Transport

	listener *minecraft.Listener
	registry *session.Registry

	logger *slog.Logger
	opts   util.Opts
}

// NewSpectrum creates a new Spectrum instance using the provided server.Discovery.
// It initializes opts with default options from util.DefaultOpts() if opts is nil,
// and defaults to TCP transport if transport is nil transport.TCP.
func NewSpectrum(discovery server.Discovery, logger *slog.Logger, opts *util.Opts, transport tr.Transport) *Spectrum {
	if opts == nil {
		opts = util.DefaultOpts()
	}

	if transport == nil {
		transport = tr.NewSpectral(logger)
	}
	return &Spectrum{
		discovery: discovery,
		transport: transport,

		registry: session.NewRegistry(),

		logger: logger,
		opts:   *opts,
	}
}

// Listen sets up a minecraft.Listener for incoming connections based on the provided minecraft.ListenConfig.
// The listener is then used by the Accept() method for accepting incoming connections.
func (s *Spectrum) Listen(config minecraft.ListenConfig) (err error) {
	listener, err := config.Listen("raknet", s.opts.Addr)
	if err != nil {
		s.logger.Error("failed to listen", "err", err)
		return err
	}
	s.listener = listener
	s.logger.Info("started listening", "addr", listener.Addr())
	return nil
}

// Accept accepts an incoming minecraft.Conn and creates a new session for it.
// This method should be called in a loop to continuously accept new connections.
func (s *Spectrum) Accept() (*session.Session, error) {
	c, err := s.listener.Accept()
	if err != nil {
		s.logger.Error("failed to accept session", "err", err)
		return nil, err
	}

	conn := c.(*minecraft.Conn)
	identityData := conn.IdentityData()
	logger := s.logger.With("username", identityData.DisplayName)
	newSession := session.NewSession(conn, logger, s.registry, s.discovery, s.opts, s.transport)
	if s.opts.AutoLogin {
		go func() {
			if err := newSession.Login(); err != nil {
				newSession.Disconnect(err.Error())
				if !errors.Is(err, context.Canceled) {
					logger.Error("failed to login session", "err", err)
				}
			}
		}()
	}
	logger.Info("accepted session")
	return newSession, nil
}

// Discovery returns the server discovery instance.
func (s *Spectrum) Discovery() server.Discovery {
	return s.discovery
}

// Opts returns the configuration options.
func (s *Spectrum) Opts() util.Opts {
	return s.opts
}

// Listener returns the listener instance.
func (s *Spectrum) Listener() *minecraft.Listener {
	return s.listener
}

// Registry returns the session registry instance.
func (s *Spectrum) Registry() *session.Registry {
	return s.registry
}

// Transport returns the transport instance.
func (s *Spectrum) Transport() tr.Transport {
	return s.transport
}

// Close closes the listener and stops listening for incoming connections.
func (s *Spectrum) Close() error {
	for _, activeSession := range s.registry.GetSessions() {
		activeSession.Disconnect(s.opts.ShutdownMessage)
	}
	return s.listener.Close()
}
