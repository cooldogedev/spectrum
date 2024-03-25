package main

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
	"github.com/spectrum-proxy/spectrum"
	"github.com/spectrum-proxy/spectrum/api"
	"github.com/spectrum-proxy/spectrum/server"
)

func main() {
	logger := logrus.New()
	listenConfig := minecraft.ListenConfig{
		StatusProvider: spectrum.NewStatusProvider("Spectrum Proxy"),
	}

	s := spectrum.NewSpectrum(server.NewStaticDiscovery(":19133"), logger, nil)
	if err := s.Listen(listenConfig); err != nil {
		logger.Errorf("Failed to listen on s: %v", err)
		return
	}

	a := api.NewAPI(logger, s.Registry())
	if err := a.Listen(":19134"); err != nil {
		logger.Errorf("Failed to listen on a: %v", err)
		return
	}

	go func() {
		for {
			if err := a.Accept(); err != nil {
				logger.Errorf("Failed to accept connection: %v", err)
			}
		}
	}()

	for {
		s, err := s.Accept()
		if err != nil {
			if s != nil {
				s.Disconnect(err.Error())
			}
			logger.Errorf("Failed to accept session: %v", err)
		}
	}
}
