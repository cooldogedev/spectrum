package animation

import (
	"image/color"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Ease represents an easing camera animation that moves the player's camera upwards in five steps,
// each step moving 25 blocks and pausing for 300 milliseconds before the next movement.
// After the animation concludes, it smoothly returns the camera downward to the player's character.
type Ease struct {
	cameraAnimation

	Flicker bool
	Colour  color.RGBA
	Timing  protocol.CameraFadeTimeData

	Position mgl32.Vec3
	Yaw      float32
}

// Play ...
func (animation *Ease) Play(conn *minecraft.Conn, _ minecraft.GameData) {
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

	time.Sleep(time.Millisecond * 300)
	for i := 0; i < 4; i++ {
		_ = conn.WritePacket(&packet.CameraInstruction{
			Set: protocol.Option(protocol.CameraInstructionSet{
				Preset: 0,
				Ease: protocol.Option(protocol.CameraEase{
					Duration: 0.3,
					Type:     protocol.EasingTypeOutExpo,
				}),
				Position: protocol.Option(animation.Position.Add(mgl32.Vec3{0, float32(25 * (i + 1)), 0})),
				Rotation: protocol.Option(mgl32.Vec2{90, animation.Yaw}),
			}),
		})
		time.Sleep(time.Millisecond * 300)
		if i != 3 && animation.Flicker {
			_ = conn.WritePacket(&packet.CameraInstruction{
				Fade: protocol.Option(protocol.CameraInstructionFade{
					TimeData: protocol.Option(protocol.CameraFadeTimeData{
						FadeInDuration:  0.1,
						WaitDuration:    0.1,
						FadeOutDuration: 0.1,
					}),
					Colour: protocol.Option(animation.Colour),
				}),
			})
			time.Sleep(time.Millisecond * 300)
		}
	}
	time.Sleep(time.Second * 3)
}

// Clear ....
func (animation *Ease) Clear(conn *minecraft.Conn, serverGameData minecraft.GameData) {
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
