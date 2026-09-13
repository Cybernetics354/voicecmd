package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"
)

var (
	iconIdle         []byte
	iconRecording    []byte
	iconTranscribing []byte
	iconInitOnce     sync.Once
)

func init() {
	initIcons()
}

func initIcons() {
	iconInitOnce.Do(func() {
		// Idle: Slate circular badge with vivid cyan/blue microphone
		idleBg := color.RGBA{R: 30, G: 41, B: 59, A: 240}  // #1e293b
		idleFg := color.RGBA{R: 56, G: 189, B: 248, A: 255} // #38bdf8
		iconIdle = generatePNG(idleBg, idleFg, nil)

		// Recording: Vibrant red badge with crisp white microphone and bright accent dot
		recBg := color.RGBA{R: 220, G: 38, B: 38, A: 255}   // #dc2626
		recFg := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		recDot := color.RGBA{R: 254, G: 240, B: 138, A: 255} // yellow dot
		iconRecording = generatePNG(recBg, recFg, &recDot)

		// Transcribing: Amber/orange badge with crisp white microphone and accent dot
		transBg := color.RGBA{R: 217, G: 119, B: 6, A: 255} // #d97706
		transFg := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		transDot := color.RGBA{R: 253, G: 230, B: 138, A: 255}
		iconTranscribing = generatePNG(transBg, transFg, &transDot)
	})
}

// IconForState returns the pre-rendered PNG icon bytes for the specified state.
func IconForState(state State) []byte {
	initIcons()
	switch state {
	case StateRecording:
		return iconRecording
	case StateTranscribing:
		return iconTranscribing
	default:
		return iconIdle
	}
}

// generatePNG creates a 64x64 PNG image containing a circular badge with a microphone icon.
func generatePNG(bgCol color.RGBA, fgCol color.RGBA, badgeCol *color.RGBA) []byte {
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Draw smooth circular background badge
	cx, cy := 32.0, 32.0
	r := 28.0

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			// Antialiased circular border
			if dist <= r+1.0 {
				alpha := 1.0
				if dist > r-1.0 {
					alpha = (r + 1.0 - dist) / 2.0
				}
				if alpha > 0 {
					c := color.RGBA{
						R: uint8(float64(bgCol.R) * alpha),
						G: uint8(float64(bgCol.G) * alpha),
						B: uint8(float64(bgCol.B) * alpha),
						A: uint8(float64(bgCol.A) * alpha),
					}
					img.Set(x, y, c)
				}
			}
		}
	}

	// Draw microphone silhouette
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5

			distMic := micDist(px, py)
			if distMic < 1.0 {
				alpha := 1.0
				if distMic > -1.0 {
					alpha = (1.0 - distMic) / 2.0
				}
				bg := img.RGBAAt(x, y)
				fgA := float64(fgCol.A) / 255.0 * alpha
				bgA := float64(bg.A) / 255.0 * (1.0 - fgA)
				outA := fgA + bgA
				if outA > 0 {
					outR := (float64(fgCol.R)*fgA + float64(bg.R)*bgA) / outA
					outG := (float64(fgCol.G)*fgA + float64(bg.G)*bgA) / outA
					outB := (float64(fgCol.B)*fgA + float64(bg.B)*bgA) / outA
					img.Set(x, y, color.RGBA{
						R: uint8(outR),
						G: uint8(outG),
						B: uint8(outB),
						A: uint8(outA * 255.0),
					})
				}
			}
		}
	}

	// If badgeCol is provided, draw small accent dot at top-right (e.g. recording / busy indicator)
	if badgeCol != nil {
		bx, by := 46.0, 18.0
		br := 6.0
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				dx := float64(x) + 0.5 - bx
				dy := float64(y) + 0.5 - by
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist <= br+0.8 {
					alpha := 1.0
					if dist > br-0.8 {
						alpha = (br + 0.8 - dist) / 1.6
					}
					bg := img.RGBAAt(x, y)
					fgA := float64(badgeCol.A) / 255.0 * alpha
					bgA := float64(bg.A) / 255.0 * (1.0 - fgA)
					outA := fgA + bgA
					if outA > 0 {
						outR := (float64(badgeCol.R)*fgA + float64(bg.R)*bgA) / outA
						outG := (float64(badgeCol.G)*fgA + float64(bg.G)*bgA) / outA
						outB := (float64(badgeCol.B)*fgA + float64(bg.B)*bgA) / outA
						img.Set(x, y, color.RGBA{
							R: uint8(outR),
							G: uint8(outG),
							B: uint8(outB),
							A: uint8(outA * 255.0),
						})
					}
				}
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// micDist returns signed distance to microphone shape (<0 inside, >0 outside)
func micDist(x, y float64) float64 {
	// 1. Capsule (rounded rectangle: top semicircular head, vertical body, bottom semicircle)
	segY := math.Max(20.0, math.Min(31.0, y))
	capsuleDist := math.Sqrt((x-32.0)*(x-32.0)+(y-segY)*(y-segY)) - 6.5

	// 2. Cradle (pickup arc U-shape around bottom of capsule)
	cradleDist := 999.0
	if y >= 25.0 {
		arcDist := math.Abs(math.Sqrt((x-32.0)*(x-32.0)+(y-28.0)*(y-28.0))-11.5) - 1.3
		if y < 26.0 {
			tipDist1 := math.Sqrt((x-20.5)*(x-20.5)+(y-25.0)*(y-25.0)) - 1.3
			tipDist2 := math.Sqrt((x-43.5)*(x-43.5)+(y-25.0)*(y-25.0)) - 1.3
			cradleDist = math.Min(arcDist, math.Min(tipDist1, tipDist2))
		} else {
			cradleDist = arcDist
		}
	}

	// 3. Stem (vertical bar from bottom of cradle down to base)
	stemDist := 999.0
	if y >= 39.0 && y <= 46.0 {
		stemY := math.Max(39.5, math.Min(45.5, y))
		stemDist = math.Sqrt((x-32.0)*(x-32.0)+(y-stemY)*(y-stemY)) - 1.4
	}

	// 4. Base (horizontal stand base bar)
	baseDist := 999.0
	if y >= 44.5 && y <= 49.0 {
		baseX := math.Max(24.0, math.Min(40.0, x))
		baseDist = math.Sqrt((x-baseX)*(x-baseX)+(y-47.0)*(y-47.0)) - 1.4
	}

	return math.Min(capsuleDist, math.Min(cradleDist, math.Min(stemDist, baseDist)))
}
