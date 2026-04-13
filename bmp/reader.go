// Package bmp provides an uncompressed BMP decoder supporting both
// standard and XOR-obfuscated files (key 0xCD), in 8-bit indexed
// and 24-bit true-color formats.
package bmp

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
)

// ColorKeyIndex is the palette index used as the transparent colorkey
// across all Block*.bmp files (confirmed from lc.c: Sprite_SetColorKey).
const ColorKeyIndex = 254

// ColorKeyRGB is the RGB color corresponding to palette index 254.
// Used as the transparency key for 24-bit BMPs (R=79, G=55, B=74 / #4f374a).
var ColorKeyRGB = color.RGBA{R: 79, G: 55, B: 74, A: 255}

// xorKey is the single-byte XOR key used by the Capone/PICS BMP files.
const xorKey = 0xCD

// Read decodes an uncompressed BMP from disk.
// Supports 8-bit (paletted, colorkey 254) and 24-bit (true-color) BMPs.
// Automatically detects and decodes XOR-obfuscated files (key 0xCD).
func Read(path string) (*image.RGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 54 {
		return nil, fmt.Errorf("file too small for BMP header")
	}

	// Auto-detect XOR obfuscation: if first two bytes aren't "BM",
	// try decoding with XOR key 0xCD.
	if data[0] != 'B' || data[1] != 'M' {
		if data[0]^xorKey == 'B' && data[1]^xorKey == 'M' {
			for i := range data {
				data[i] ^= xorKey
			}
		} else {
			return nil, fmt.Errorf("not a valid BMP file")
		}
	}

	pixelOffset := int(binary.LittleEndian.Uint32(data[10:14]))
	dibSize := int(binary.LittleEndian.Uint32(data[14:18]))
	if dibSize < 40 {
		return nil, fmt.Errorf("unsupported DIB header (size %d)", dibSize)
	}

	width := int(int32(binary.LittleEndian.Uint32(data[18:22])))
	height := int(int32(binary.LittleEndian.Uint32(data[22:26])))
	bpp := binary.LittleEndian.Uint16(data[28:30])
	compression := binary.LittleEndian.Uint32(data[30:34])

	if bpp != 8 && bpp != 24 {
		return nil, fmt.Errorf("unsupported bit depth %d (expected 8 or 24)", bpp)
	}
	if compression != 0 {
		return nil, fmt.Errorf("unsupported compression %d (expected 0)", compression)
	}

	bottomUp := height > 0
	if height < 0 {
		height = -height
	}

	if bpp == 8 {
		return read8bpp(data, width, height, dibSize, pixelOffset, bottomUp)
	}
	return read24bpp(data, width, height, pixelOffset, bottomUp)
}

// read8bpp decodes an 8-bit paletted BMP. Palette index 254 is transparent.
func read8bpp(data []byte, width, height, dibSize, pixelOffset int, bottomUp bool) (*image.RGBA, error) {
	// Palette starts right after the DIB header
	palOff := 14 + dibSize
	palette := [256]color.RGBA{}
	for i := 0; i < 256; i++ {
		off := palOff + i*4
		if off+3 >= len(data) {
			break
		}
		// RGBQUAD layout: blue, green, red, reserved.
		// Go's image/draw uses pre-multiplied alpha, so transparent
		// pixels must have R=G=B=0 to avoid color bleed during compositing.
		if i == ColorKeyIndex {
			palette[i] = color.RGBA{}
		} else {
			palette[i] = color.RGBA{R: data[off+2], G: data[off+1], B: data[off+0], A: 255}
		}
	}

	// Pixel rows are padded to 4-byte boundaries
	stride := (width + 3) &^ 3
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		srcRow := y
		if bottomUp {
			srcRow = height - 1 - y
		}
		rowOff := pixelOffset + srcRow*stride
		for x := 0; x < width; x++ {
			if rowOff+x >= len(data) {
				break
			}
			img.SetRGBA(x, y, palette[data[rowOff+x]])
		}
	}

	return img, nil
}

// read24bpp decodes a 24-bit true-color BMP (BGR byte order, no palette).
// Pixels matching ColorKeyRGB are made fully transparent.
func read24bpp(data []byte, width, height, pixelOffset int, bottomUp bool) (*image.RGBA, error) {
	// Row stride: 3 bytes per pixel, padded to 4-byte boundary
	stride := (width*3 + 3) &^ 3
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	ckR, ckG, ckB := ColorKeyRGB.R, ColorKeyRGB.G, ColorKeyRGB.B

	for y := 0; y < height; y++ {
		srcRow := y
		if bottomUp {
			srcRow = height - 1 - y
		}
		rowOff := pixelOffset + srcRow*stride
		for x := 0; x < width; x++ {
			off := rowOff + x*3
			if off+2 >= len(data) {
				break
			}
			// BMP 24-bit pixel layout: Blue, Green, Red
			b, g, r := data[off], data[off+1], data[off+2]
			if r == ckR && g == ckG && b == ckB {
				// Transparent — leave as zero (pre-multiplied alpha)
				continue
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img, nil
}
