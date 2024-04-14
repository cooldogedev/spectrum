package main

import (
	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/api"
	"github.com/cooldogedev/spectrum/server"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
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

	a := api.NewAPI(s.Registry(), logger, nil)
	if err := a.Listen(":19132"); err != nil {
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
