// mono turns the full-colour Mouse Without Borders icon into a one-colour
// mask for tinting: the back screen becomes 50% opaque, the front screen fully
// opaque, and the (near white) cursor fully transparent. Pixels are white; the
// plugin tints them with the bar's foreground colour.
//
//	go run ./tools/mono assets/mwb.png assets/mwb-mono.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// alphaFor maps luminance (0-255) to the output opacity.
//
//	back screen  ~73-90   -> 0.5
//	front screen ~125-135 -> 1.0
//	cursor       ~235+    -> 0.0
func alphaFor(l float64) float64 {
	pts := [][2]float64{{0, 0.5}, {95, 0.5}, {120, 1}, {185, 1}, {225, 0}, {255, 0}}
	for i := 1; i < len(pts); i++ {
		if l <= pts[i][0] {
			t := (l - pts[i-1][0]) / (pts[i][0] - pts[i-1][0])
			return pts[i-1][1] + t*(pts[i][1]-pts[i-1][1])
		}
	}
	return 0
}

func main() {
	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	src, err := png.Decode(in)
	if err != nil {
		panic(err)
	}
	b := src.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			l := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			a := alphaFor(l) * float64(c.A) / 255
			out.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a*255 + 0.5)})
		}
	}
	f, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		panic(err)
	}
}
