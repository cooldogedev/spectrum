package spectrum

import (
	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	tr "github.com/cooldogedev/spectrum/transport"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
)

type Spectrum struct {
	discovery server.Discovery
	transport tr.Transport

	listener *minecraft.Listener
	registry *session.Registry

	logger internal.Logger
	opts   *util.Opts
}

func NewSpectrum(discovery server.Discovery, logger internal.Logger, opts *util.Opts, transport tr.Transport) *Spectrum {
	if opts == nil {
		opts = util.DefaultOpts()
	}

	if transport == nil {
		transport = tr.NewTCP()
	}
	return &Spectrum{
		discovery: discovery,
		transport: transport,

		registry: session.NewRegistry(),

		logger: logger,
		opts:   opts,
	}
}

func (s *Spectrum) Listen(config minecraft.ListenConfig) (err error) {
	listener, err := config.Listen("raknet", s.opts.Addr)
	if err != nil {
		s.logger.Errorf("Failed to start s: %v", err)
		return err
	}

	s.listener = listener
	s.logger.Infof("Started sprectrum on %v", listener.Addr())
	return nil
}

func (s *Spectrum) Accept() (*session.Session, error) {
	conn, err := s.listener.Accept()
	if err != nil {
		s.logger.Errorf("Failed to accept session: %v", err)
		return nil, err
	}

	newSession, err := session.NewSession(conn.(*minecraft.Conn), s.logger, s.discovery, s.opts, s.registry, s.transport)
	if err != nil {
		s.logger.Errorf("Failed to create session: %v", err)
		return nil, err
	}

	s.logger.Debugf("Accepted session from %v", conn.RemoteAddr())
	return newSession, nil
}

func (s *Spectrum) Discovery() server.Discovery {
	return s.discovery
}

func (s *Spectrum) Opts() *util.Opts {
	return s.opts
}

func (s *Spectrum) Registry() *session.Registry {
	return s.registry
}

func (s *Spectrum) Transport() tr.Transport {
	return s.transport
}

func (s *Spectrum) Close() error {
	return s.listener.Close()
}
