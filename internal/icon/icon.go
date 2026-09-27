package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

type State int

const (
	StateFull State = iota
	StateMedium
	StateLow
	StateUnknown // no controller connected: drawn like a full battery, but dark gray
)

var fillColor = map[State]color.RGBA{
	StateFull:    {0x2E, 0xCC, 0x71, 0xFF}, // green
	StateMedium:  {0xF1, 0xC4, 0x0F, 0xFF}, // yellow
	StateLow:     {0xE7, 0x4C, 0x3C, 0xFF}, // red
	StateUnknown: {0x5A, 0x5A, 0x5A, 0xFF}, // dark gray
}

// fillRatio controls how much of the battery body is drawn as filled
var fillRatio = map[State]float64{
	StateFull:    1.0,
	StateMedium:  0.6,
	StateLow:     0.25,
	StateUnknown: 1.0,
}

const Size = 32

// Render draws a 32x32 battery icon for the given state
func Render(s State) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Size, Size))

	// transparent background
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 0})
		}
	}

	outline := color.RGBA{0xEC, 0xF0, 0xF1, 0xFF}
	fill := fillColor[s]

	// battery body: horizontal, with a small "nub" (positive terminal) on the right
	bodyX0, bodyY0 := 2, 8
	bodyX1, bodyY1 := 26, 24
	nubX0, nubX1 := 26, 29
	nubY0, nubY1 := 12, 20

	drawRectOutline(img, bodyX0, bodyY0, bodyX1, bodyY1, outline, 2)
	drawFilledRect(img, nubX0, nubY0, nubX1, nubY1, outline)

	ratio := fillRatio[s]
	if ratio > 0 {
		innerX0, innerY0 := bodyX0+3, bodyY0+3
		innerX1, innerY1 := bodyX1-3, bodyY1-3
		fullWidth := innerX1 - innerX0
		fillWidth := int(float64(fullWidth) * ratio)
		drawFilledRect(img, innerX0, innerY0, innerX0+fillWidth, innerY1, fill)
	}

	return img
}

func drawRectOutline(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	for t := 0; t < thickness; t++ {
		for x := x0; x <= x1; x++ {
			img.Set(x, y0+t, c)
			img.Set(x, y1-t, c)
		}
		for y := y0; y <= y1; y++ {
			img.Set(x0+t, y, c)
			img.Set(x1-t, y, c)
		}
	}
}

func drawFilledRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			img.Set(x, y, c)
		}
	}
}

// EncodePNG encodes the image as PNG bytes
func EncodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// EncodeICO wraps a single PNG frame in a minimal .ico container
func EncodeICO(img image.Image) []byte {
	pngBytes := EncodePNG(img)

	var buf bytes.Buffer
	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type = icon
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // count

	// ICONDIRENTRY
	buf.WriteByte(byte(Size))                           // width
	buf.WriteByte(byte(Size))                           // height
	buf.WriteByte(0)                                    // color count
	buf.WriteByte(0)                                    // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32)) // bit count
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngBytes)))
	binary.Write(&buf, binary.LittleEndian, uint32(6+16)) // offset: header(6)+entry(16)

	buf.Write(pngBytes)
	return buf.Bytes()
}
