package main

import (
	"math"
	"math/rand"
)

const (
	faceColor = 0x14181F

	eyeRadius     = 16
	eyeOffsetX    = 110
	eyeY          = 150
	eyeArchHeight = 14 // how tall the closed-eye arch bulges
	eyeLineWidth  = 8  // stroke thickness while closed

	blinkDuration  = 10  // frames for a full close+open cycle
	blinkMinPeriod = 120 // min frames between blinks
	blinkMaxPeriod = 300 // max frames between blinks

	maxLookOffset = 14
	lookEase      = 0.08
	lookMinPeriod = 90
	lookMaxPeriod = 240

	mouthCY         = 160
	mouthHalfWidth  = 50
	mouthArchHeight = 50 // how deep the smile dips
	mouthLineWidth  = 10
)

var (
	blinkTimer = blinkMinPeriod // frames until the next blink starts
	blinkFrame = 0              // >0 while a blink is in progress

	lookTimer                = lookMinPeriod // frames until the next gaze change
	lookX, lookY             float64
	targetLookX, targetLookY float64
)

var lookDirections = [][2]float64{
	{0, 0},
	{-1, 0}, {1, 0},
	{0, -1}, {0, 1},
	{-0.7, -0.7}, {0.7, -0.7},
	{-0.7, 0.7}, {0.7, 0.7},
}

func face() {
	updateBlink()
	updateLook()

	dx := int(lookX * maxLookOffset)
	dy := int(lookY * maxLookOffset)

	drawEye(width/2-eyeOffsetX+dx, eyeY+dy)
	drawEye(width/2+eyeOffsetX+dx, eyeY+dy)
	drawMouth()
}

func updateBlink() {
	if blinkFrame > 0 {
		blinkFrame++
		if blinkFrame > blinkDuration {
			blinkFrame = 0
			blinkTimer = randRange(blinkMinPeriod, blinkMaxPeriod)
		}
		return
	}
	blinkTimer--
	if blinkTimer <= 0 {
		blinkFrame = 1
	}
}

// blinkOpenness returns 1 when the eyes are fully open and 0 when fully closed.
func blinkOpenness() float64 {
	if blinkFrame == 0 {
		return 1
	}
	half := float64(blinkDuration) / 2
	t := float64(blinkFrame)
	closedness := min(t, float64(blinkDuration)-t) / half
	return 1 - closedness
}

func updateLook() {
	lookTimer--
	if lookTimer <= 0 {
		dir := lookDirections[rand.Intn(len(lookDirections))]
		targetLookX, targetLookY = dir[0], dir[1]
		lookTimer = randRange(lookMinPeriod, lookMaxPeriod)
	}
	lookX += (targetLookX - lookX) * lookEase
	lookY += (targetLookY - lookY) * lookEase
}

func randRange(lo, hi int) int {
	return lo + rand.Intn(hi-lo+1)
}

// drawEye morphs between a round open eye and an upward-curving closed
// arch, passing through a flat line at the midpoint of the transition —
// matching BMO's classic blink where the eye briefly becomes a simple ^ arc.
func drawEye(cx, cy int) {
	closedness := 1 - blinkOpenness()
	if closedness <= 0.5 {
		ry := max(int(float64(eyeRadius)*(1-closedness*2)), eyeLineWidth/2)
		fillEllipse(cx, cy, eyeRadius, ry, faceColor)
		return
	}
	depth := (closedness - 0.5) * 2
	drawArcStroke(cx, cy, eyeRadius, eyeArchHeight*depth, eyeLineWidth, faceColor)
}

func drawMouth() {
	drawArcStroke(width/2, mouthCY, mouthHalfWidth, -mouthArchHeight, mouthLineWidth, faceColor)
}

func fillEllipse(cx, cy, rx, ry int, color uint32) {
	if rx <= 0 || ry <= 0 {
		return
	}
	rxf, ryf := float64(rx), float64(ry)
	// the extra 1px border catches the antialiased fringe just outside the
	// hard radius, where coverage falls from partial to zero.
	for y := -ry - 1; y <= ry+1; y++ {
		fy := float64(y)
		for x := -rx - 1; x <= rx+1; x++ {
			fx := float64(x)
			alpha := ellipseCoverage(fx, fy, rxf, ryf)
			if alpha > 0 {
				blendPixel(cx+x, cy+y, color, alpha)
			}
		}
	}
}

// ellipseCoverage estimates the fraction of the pixel at offset (x,y) from
// the ellipse's center that lies inside it, from the signed distance to the
// boundary approximated via the implicit ellipse equation's gradient
// (standard analytic antialiasing for implicit shapes — cheap, no
// supersampling loop needed).
func ellipseCoverage(x, y, rx, ry float64) float64 {
	nx := x / rx
	ny := y / ry
	d2 := nx*nx + ny*ny // normalized squared distance; boundary at 1

	// The antialiased fringe is only ever ~1px wide, comfortably inside a
	// ±30% band around the boundary. Skipping the sqrt below for pixels well
	// inside or outside that band is what keeps the many overlapping stamps
	// in drawArcStroke cheap.
	const innerD2, outerD2 = 0.7 * 0.7, 1.3 * 1.3
	if d2 <= innerD2 {
		return 1
	}
	if d2 >= outerD2 {
		return 0
	}

	gx := x / (rx * rx)
	gy := y / (ry * ry)
	grad := 2 * math.Sqrt(gx*gx+gy*gy)
	if grad == 0 {
		return 1
	}
	dist := (d2 - 1) / grad // signed distance in pixels; positive = outside
	return clampF(0.5-dist, 0, 1)
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// drawArcStroke paints a rounded-cap stroke of the given thickness tracing a
// half-ellipse spanning [cx-halfWidth, cx+halfWidth] (a true semicircle when
// archHeight == halfWidth). archHeight > 0 bulges the middle upward (an
// arch, e.g. a closed eye); archHeight < 0 bulges it downward (a smile).
// archHeight == 0 draws a straight capsule.
//
// A half-ellipse spreads curvature evenly across the whole span; a sine
// profile concentrates it near the peak and reads as two straight legs
// meeting a point at tall height-to-width ratios.
//
// Sampled by angle rather than by x: near the tips the curve's tangent
// approaches vertical, so equal steps in x leave y undersampled there,
// stranding the end cap as a visible dot disconnected from the rest of the
// stroke. Equal steps in angle keep samples evenly spaced along the curve
// all the way to the tips.
func drawArcStroke(cx, cy int, halfWidth, archHeight, thickness float64, color uint32) {
	if halfWidth <= 0 {
		return
	}
	radius := int(math.Round(thickness / 2))
	// Space stamps about one radius apart along the curve (enough overlap for
	// a seamless stroke, no more). Using a flat multiplier of the curve's
	// extent regardless of thickness way oversampled thick strokes — the
	// mouth was stamping ~300 overlapping circles where ~30 fully cover it.
	arcLength := math.Pi * (halfWidth + math.Abs(archHeight)) / 2
	steps := max(int(math.Ceil(arcLength/math.Max(float64(radius), 1))), 8)
	for i := 0; i <= steps; i++ {
		theta := float64(i)/float64(steps)*math.Pi - math.Pi/2 // -pi/2..pi/2
		x := float64(cx) + halfWidth*math.Sin(theta)
		y := float64(cy) - archHeight*math.Cos(theta)
		fillEllipse(int(math.Round(x)), int(math.Round(y)), radius, radius, color)
	}
}
