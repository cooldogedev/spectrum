package main

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
	"github.com/spectrum-proxy/spectrum"
	"github.com/spectrum-proxy/spectrum/server"
)

func main() {
	logger := logrus.New()
	listenConfig := minecraft.ListenConfig{StatusProvider: spectrum.NewStatusProvider("Spectrum Proxy")}
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery(":19133"), logger, nil)
	if err := proxy.Listen(listenConfig); err != nil {
		logger.Errorf("Failed to listen on proxy: %v", err)
		return
	}

	for {
		s, err := proxy.Accept()
		if err != nil {
			if s != nil {
				s.Disconnect(err.Error())
			}
			logger.Errorf("Failed to accept session: %v", err)
		}
	}
}
