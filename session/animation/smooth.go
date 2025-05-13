package animation

import (
	"image/color"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Smooth represents a camera animation that smoothly moves the player's camera upwards towards the sky
// and returns it downward to the player's character when the animation concludes.
type Smooth struct {
	cameraAnimation

	Colour color.RGBA
	Timing protocol.CameraFadeTimeData

	Position mgl32.Vec3
	Yaw      float32
}

// Play ...
func (animation *Smooth) Play(conn *minecraft.Conn, _ minecraft.GameData) {
	animation.Sync(conn)
	_ = conn.WritePacket(&packet.CameraInstruction{
		Set: protocol.Option(protocol.CameraInstructionSet{
			Preset: 0,
			Ease: protocol.Option(protocol.CameraEase{
				Duration: 0.3,
				Type:     protocol.EasingTypeLinear,
			}),
			Position: protocol.Option(animation.Position),
			Rotation: protocol.Option(mgl32.Vec2{90, animation.Yaw}),
		}),
	})

	time.Sleep(time.Millisecond * 350)
	_ = conn.WritePacket(&packet.CameraInstruction{
		Set: protocol.Option(protocol.CameraInstructionSet{
			Preset: 0,
			Ease: protocol.Option(protocol.CameraEase{
				Duration: 2.5,
				Type:     protocol.EasingTypeOutExpo,
			}),
			Position: protocol.Option(animation.Position.Add(mgl32.Vec3{0, 100, 0})),
			Rotation: protocol.Option(mgl32.Vec2{90, animation.Yaw}),
		}),
	})
	time.Sleep(time.Second * 3)
}

// Clear ...
func (animation *Smooth) Clear(conn *minecraft.Conn, serverGameData minecraft.GameData) {
	go func() {
		timing := animation.Timing
		_ = conn.WritePacket(&packet.CameraInstruction{
			Set: protocol.Option(protocol.CameraInstructionSet{
				Preset: 0,
				Ease: protocol.Option(protocol.CameraEase{
					Duration: 0.0,
					Type:     protocol.EasingTypeLinear,
				}),
				Position: protocol.Option(serverGameData.PlayerPosition.Add(mgl32.Vec3{0, 100, 0})),
				Rotation: protocol.Option(mgl32.Vec2{90, serverGameData.Yaw}),
			}),
			Fade: protocol.Option(protocol.CameraInstructionFade{
				TimeData: protocol.Option(timing),
				Colour:   protocol.Option(animation.Colour),
			}),
		})

		time.Sleep(time.Second * time.Duration(timing.FadeInDuration+timing.WaitDuration+timing.FadeOutDuration))
		_ = conn.WritePacket(&packet.CameraInstruction{
			Set: protocol.Option(protocol.CameraInstructionSet{
				Preset: 0,
				Ease: protocol.Option(protocol.CameraEase{
					Duration: 2.75,
					Type:     protocol.EasingTypeInExpo,
				}),
				Position: protocol.Option(serverGameData.PlayerPosition.Add(mgl32.Vec3{0, 1, 0})),
				Rotation: protocol.Option(mgl32.Vec2{90, serverGameData.Yaw}),
			}),
		})

		time.Sleep(time.Second * 3)
		_ = conn.WritePacket(&packet.CameraInstruction{
			Set: protocol.Option(protocol.CameraInstructionSet{
				Preset: 0,
				Ease: protocol.Option(protocol.CameraEase{
					Duration: 0.3,
					Type:     protocol.EasingTypeInOutExpo,
				}),
				Position: protocol.Option(serverGameData.PlayerPosition.Add(yawToDirectionVec(float64(serverGameData.Yaw)))),
				Rotation: protocol.Option(mgl32.Vec2{serverGameData.Pitch, serverGameData.Yaw}),
			}),
		})

		time.Sleep(time.Millisecond * 400)
		_ = conn.WritePacket(&packet.CameraInstruction{Clear: protocol.Option(true)})
	}()
}
