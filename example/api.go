package main

import (
	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/api"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery("127.0.0.1:19133"), logger, nil, nil)
	if err := proxy.Listen(minecraft.ListenConfig{StatusProvider: util.NewStatusProvider("Spectrum Proxy", "Spectrum")}); err != nil {
		logger.Errorf("Failed to listen on proxy: %v", err)
		return
	}

	go func() {
		a := api.NewAPI(proxy.Registry(), logger, nil)
		if err := a.Listen("127.0.0.1:19132"); err != nil {
			logger.Errorf("Failed to listen on a: %v", err)
			return
		}

		for {
			if err := a.Accept(); err != nil {
				logger.Errorf("Failed to accept connection: %v", err)
			}
		}
	}()

	for {
		if _, err := proxy.Accept(); err != nil {
			logger.Errorf("Failed to accept session: %v", err)
		}
	}
}
