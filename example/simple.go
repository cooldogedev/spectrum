package main

import (
	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
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
		if _, err := proxy.Accept(); err != nil {
			logger.Errorf("Failed to accept session: %v", err)
		}
	}
}
