package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	frameRate = 60

	width  = 480
	height = 320

	frontBufferDevice = "/tmp/bmo_fb"
	//frontBufferDevice = "/dev/fb1"
)

func main() {

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic: %s\n", r)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	t := time.NewTicker(time.Second / frameRate)
	f, err := os.OpenFile(frontBufferDevice, os.O_WRONLY, 0)
	if err != nil {
		panic(fmt.Errorf("opening front buffer: %w", err))
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(fmt.Errorf("closing front buffer: %w", err))
		}
	}()
	info, err := f.Stat()
	if err != nil {
		panic(err)
	}
	isPipe := (info.Mode() & os.ModeNamedPipe) != 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			render(ctx, f, isPipe)
		}
	}
}

// color format is BGR565 16bit

var backbuffer = make([]byte, width*height*2)

func writePixel(x, y int, color uint32) {
	i := (y*width + x) * 2
	if i < 0 || i+1 >= len(backbuffer) {
		return
	}
	c := rgb2bgr565(color)
	backbuffer[(y*width+x)*2] = byte(c & 0xFF)
	backbuffer[(y*width+x)*2+1] = byte((c >> 8) & 0xFF)
}

// readPixel decodes the current backbuffer pixel back to 24-bit RGB, used to
// blend against when antialiasing shape edges.
func readPixel(x, y int) uint32 {
	if x < 0 || y < 0 || x >= width || y >= height {
		return 0
	}
	i := (y*width + x) * 2
	if i < 0 || i+1 >= len(backbuffer) {
		return 0
	}
	c := uint16(backbuffer[i]) | uint16(backbuffer[i+1])<<8
	b5 := (c >> 11) & 0x1F
	g6 := (c >> 5) & 0x3F
	r5 := c & 0x1F
	r8 := uint32((r5 << 3) | (r5 >> 2))
	g8 := uint32((g6 << 2) | (g6 >> 4))
	b8 := uint32((b5 << 3) | (b5 >> 2))
	return r8<<16 | g8<<8 | b8
}

// blendPixel writes color over whatever is already at (x,y), weighted by
// alpha in [0,1] — the basis for edge antialiasing.
func blendPixel(x, y int, color uint32, alpha float64) {
	if alpha >= 1 {
		writePixel(x, y, color)
		return
	}
	if alpha <= 0 {
		return
	}
	writePixel(x, y, lerpColor(readPixel(x, y), color, alpha))
}

func lerpColor(a, b uint32, t float64) uint32 {
	ar, ag, ab := float64(a>>16&0xFF), float64(a>>8&0xFF), float64(a&0xFF)
	br, bg, bb := float64(b>>16&0xFF), float64(b>>8&0xFF), float64(b&0xFF)
	r := uint32(ar + (br-ar)*t + 0.5)
	g := uint32(ag + (bg-ag)*t + 0.5)
	bl := uint32(ab + (bb-ab)*t + 0.5)
	return r<<16 | g<<8 | bl
}

func quantize5(v uint8) uint16 {
	return uint16((uint32(v)*31 + 128) / 255)
}

func quantize6(v uint8) uint16 {
	return uint16((uint32(v)*63 + 128) / 255)
}

func rgb2bgr565(c uint32) uint16 {
	r := uint8(c >> 16 & 0xFF)
	g := uint8(c >> 8 & 0xFF)
	b := uint8(c & 0xFF)
	rq := quantize5(r)
	gq := quantize6(g)
	bq := quantize5(b)

	return (bq << 11) | (gq << 5) | rq
}

func render(ctx context.Context, frontbuffer io.WriteSeeker, isPipe bool) {
	defer func() {
		if _, err := frontbuffer.Write(backbuffer); err != nil {
			panic(fmt.Errorf("swapping buffers: %w", err))
		}
		if !isPipe {
			_, err := frontbuffer.Seek(0, 0)
			if err != nil {
				panic(fmt.Errorf("seeking start: %w", err))
			}
		}
	}()

	background()
	face()
}

func background() {
	for y := range height {
		for x := range width {
			writePixel(x, y, 0xCFF6D5)
		}
	}
}
