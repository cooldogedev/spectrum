package animation

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

func yawToDirectionVec(yaw float64) mgl32.Vec3 {
	yaw = math.Mod(yaw, 360)
	if yaw < 0 {
		yaw += 360
	}

	radians := yaw * (math.Pi / 180)
	x := math.Cos(radians)
	y := math.Sin(radians)
	if y >= 0 && math.Abs(x) <= 0.5 {
		return mgl32.Vec3{-1, 0, 0}
	} else if x >= 0 && math.Abs(y) <= 0.5 {
		return mgl32.Vec3{0, 0, 1}
	} else if y < 0 && math.Abs(x) <= 0.5 {
		return mgl32.Vec3{1, 0, 0}
	} else {
		return mgl32.Vec3{0, 0, -1}
	}
}
