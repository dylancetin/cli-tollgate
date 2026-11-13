// Package ico encodes Windows .ico files and draws the tray icon variants.
package ico

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"math"
	"sort"
)

// Encode writes the images as a classic DIB-based .ico (32-bit BGRA plus an
// empty AND mask). Every Windows version loads this form through LoadImage,
// unlike PNG-compressed entries.
func Encode(images []image.Image) ([]byte, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("ico: no images")
	}
	type entry struct {
		w, h int
		data []byte
	}
	entries := make([]entry, 0, len(images))
	for _, img := range images {
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()
		if w <= 0 || h <= 0 || w > 256 || h > 256 {
			return nil, fmt.Errorf("ico: unsupported size %dx%d", w, h)
		}
		entries = append(entries, entry{w, h, dib(img)})
	}

	var out bytes.Buffer
	le := binary.LittleEndian
	header := make([]byte, 6)
	le.PutUint16(header[2:], 1) // type: icon
	le.PutUint16(header[4:], uint16(len(entries)))
	out.Write(header)

	offset := 6 + 16*len(entries)
	for _, e := range entries {
		dir := make([]byte, 16)
		dir[0] = byte(e.w % 256) // 0 means 256
		dir[1] = byte(e.h % 256)
		le.PutUint16(dir[4:], 1)  // planes
		le.PutUint16(dir[6:], 32) // bits per pixel
		le.PutUint32(dir[8:], uint32(len(e.data)))
		le.PutUint32(dir[12:], uint32(offset))
		out.Write(dir)
		offset += len(e.data)
	}
	for _, e := range entries {
		out.Write(e.data)
	}
	return out.Bytes(), nil
}

func dib(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	maskStride := ((w + 31) / 32) * 4
	le := binary.LittleEndian

	var buf bytes.Buffer
	hdr := make([]byte, 40)
	le.PutUint32(hdr[0:], 40)
	le.PutUint32(hdr[4:], uint32(w))
	le.PutUint32(hdr[8:], uint32(h*2)) // XOR + AND masks
	le.PutUint16(hdr[12:], 1)
	le.PutUint16(hdr[14:], 32)
	le.PutUint32(hdr[20:], uint32(w*h*4+maskStride*h))
	buf.Write(hdr)

	px := make([]byte, 4)
	for y := h - 1; y >= 0; y-- { // bottom-up rows
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			px[0], px[1], px[2], px[3] = c.B, c.G, c.R, c.A
			buf.Write(px)
		}
	}
	buf.Write(make([]byte, maskStride*h)) // alpha channel carries transparency
	return buf.Bytes()
}

// State selects the tray icon variant.
type State int

const (
	Ready      State = iota // running, at least one account signed in
	NeedsLogin              // running, no accounts yet
	Busy                    // starting
	Off                     // stopped
	Problem                 // failed
)

//go:embed tray/*.png
var trayFS embed.FS

var trayImages = loadTray()

func loadTray() []image.Image {
	names, _ := fs.Glob(trayFS, "tray/*.png")
	var imgs []image.Image
	for _, n := range names {
		f, err := trayFS.Open(n)
		if err != nil {
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err == nil {
			imgs = append(imgs, img)
		}
	}
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].Bounds().Dx() < imgs[j].Bounds().Dx() })
	return imgs
}

// TrayIcon returns a multi-size .ico for the given state: the app icon,
// greyed out when the proxy is not running, with a status dot when the user
// needs to act.
func TrayIcon(s State) []byte {
	var frames []image.Image
	for _, base := range trayImages {
		frames = append(frames, TrayFrame(base, s))
	}
	data, err := Encode(frames)
	if err != nil {
		return nil
	}
	return data
}

// TrayFrame draws one size of the tray icon for the given state.
func TrayFrame(base image.Image, s State) image.Image {
	b := base.Bounds()
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), base, b.Min, draw.Src)

	if s == Busy || s == Off {
		desaturate(img, s == Off)
	}
	switch s {
	case NeedsLogin:
		dot(img, color.NRGBA{0xf5, 0x9e, 0x0b, 0xff}) // amber
	case Problem:
		dot(img, color.NRGBA{0xef, 0x44, 0x44, 0xff}) // red
	}
	return img
}

func desaturate(img *image.NRGBA, dim bool) {
	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b := float64(img.Pix[i]), float64(img.Pix[i+1]), float64(img.Pix[i+2])
		l := 0.299*r + 0.587*g + 0.114*b
		if dim {
			l = l*0.75 + 40
		}
		v := uint8(math.Min(255, l))
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = v, v, v
	}
}

// dot paints an anti-aliased status dot in the bottom-right corner, with a
// transparent gap around it so it reads on light and dark taskbars.
func dot(img *image.NRGBA, c color.NRGBA) {
	size := float64(img.Bounds().Dx())
	r := size * 0.24
	gap := math.Max(1, size*0.07)
	cx, cy := size-r-0.5, size-r-0.5
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			p := img.Pix[img.PixOffset(x, y):][:4]
			if keep := clamp(d - (r + gap) + 0.5); keep < 1 {
				p[3] = uint8(float64(p[3]) * keep)
			}
			if cov := clamp(r - d + 0.5); cov > 0 {
				over(p, c, cov)
			}
		}
	}
}

func clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// over composites colour c with coverage a over the straight-alpha pixel p.
func over(p []byte, c color.NRGBA, a float64) {
	pa := float64(p[3]) / 255
	outA := a + pa*(1-a)
	if outA <= 0 {
		return
	}
	mix := func(src, dst byte) byte {
		return uint8((float64(src)*a + float64(dst)*pa*(1-a)) / outA)
	}
	p[0], p[1], p[2] = mix(c.R, p[0]), mix(c.G, p[1]), mix(c.B, p[2])
	p[3] = uint8(outA * 255)
}
