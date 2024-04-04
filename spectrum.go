package spectrum

import (
	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	"github.com/sandertv/gophertunnel/minecraft"
)

type Spectrum struct {
	logger   internal.Logger
	registry *session.Registry

	listener  *minecraft.Listener
	discovery server.Discovery
	opts      *Opts
}

func NewSpectrum(discovery server.Discovery, logger internal.Logger, opts *Opts) *Spectrum {
	if opts == nil {
		opts = DefaultOpts()
	}

	return &Spectrum{
		logger:   logger,
		registry: session.NewRegistry(),

		discovery: discovery,
		opts:      opts,
	}
}

func (s *Spectrum) Listen(config minecraft.ListenConfig) (err error) {
	listener, err := config.Listen("raknet", s.opts.Addr)
	if err != nil {
		s.logger.Errorf("Failed to start s: %v", err)
		return err
	}

	s.logger.Infof("Started sprectrum on %v", listener.Addr())
	s.listener = listener
	return nil
}

func (s *Spectrum) Accept() (*session.Session, error) {
	conn, err := s.listener.Accept()
	if err != nil {
		s.logger.Errorf("Failed to accept session: %v", err)
		return nil, err
	}

	newSession, err := session.NewSession(conn.(*minecraft.Conn), s.logger, s.registry, s.discovery, s.opts.LatencyInterval)
	if err != nil {
		s.logger.Errorf("Failed to create session: %v", err)
		return nil, err
	}

	s.logger.Debugf("Accepted session from %v", conn.RemoteAddr())
	return newSession, nil
}

func (s *Spectrum) Close() error {
	return s.listener.Close()
}

func (s *Spectrum) Registry() *session.Registry {
	return s.registry
}
