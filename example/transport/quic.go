package main

import (
	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/transport"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery("10.244.4.97:19132"), logger, nil, transport.NewQUIC(logger))
	if err := proxy.Listen(minecraft.ListenConfig{StatusProvider: util.NewStatusProvider("Spectrum Proxy")}); err != nil {
		logger.Errorf("Failed to listen on proxy: %v", err)
		return
	}

	for {
		if _, err := proxy.Accept(); err != nil {
			logger.Errorf("Failed to accept session: %v", err)
		}
	}
}
