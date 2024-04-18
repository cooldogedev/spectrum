package main

import (
	"image/color"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session/animation"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
		s, err := proxy.Accept()
		if err != nil {
			logger.Errorf("Failed to accept session: %v", err)
			continue
		}

		s.SetAnimation(&animation.Fade{
			Colour: color.RGBA{},
			Timing: protocol.CameraFadeTimeData{
				FadeInDuration:  0.5,
				WaitDuration:    5.0,
				FadeOutDuration: 0.5,
			},
		})
	}
}
