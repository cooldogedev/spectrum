package main

import (
	"image/color"
	"log/slog"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	"github.com/cooldogedev/spectrum/session/animation"
	"github.com/cooldogedev/spectrum/util"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type easeProcessor struct {
	session.NopProcessor
	consumer func(mgl32.Vec3, float32)
}

func (p *easeProcessor) ProcessClient(_ *session.Context, pk packet.Packet) {
	if pk, ok := pk.(*packet.PlayerAuthInput); ok {
		p.consumer(pk.Position.Add(mgl32.Vec3{0, 1, 0}), pk.HeadYaw)
	}
}

func main() {
	logger := slog.Default()
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery("127.0.0.1:19133", ""), logger, nil, nil)
	if err := proxy.Listen(minecraft.ListenConfig{StatusProvider: util.NewStatusProvider("Spectrum Proxy", "Spectrum")}); err != nil {
		return
	}

	for {
		s, err := proxy.Accept()
		if err != nil {
			continue
		}

		s.SetAnimation(&animation.Ease{
			Flicker: true,
			Colour:  color.RGBA{},
			Timing: protocol.CameraFadeTimeData{
				FadeInDuration:  0.75,
				WaitDuration:    3.25,
				FadeOutDuration: 0.75,
			},
		})
		s.SetProcessor(&easeProcessor{consumer: func(position mgl32.Vec3, yaw float32) {
			s.Animation().(*animation.Ease).Position = position
			s.Animation().(*animation.Ease).Yaw = yaw
		}})
	}
}
