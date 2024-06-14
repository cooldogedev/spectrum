package main

import (
	"image/color"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session/animation"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery("127.0.0.1:19133"), logger, nil, nil)
	if err := proxy.Listen(minecraft.ListenConfig{StatusProvider: util.NewStatusProvider("Spectrum Proxy", "Spectrum")}); err != nil {
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
				FadeInDuration:  0.25,
				WaitDuration:    4.50,
				FadeOutDuration: 0.25,
			},
		})
	}
}
