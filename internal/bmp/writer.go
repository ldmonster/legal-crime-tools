package bmp

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"sort"
)

// WriteOldFormat writes img to path as an unobfuscated 8-bit indexed BMP
// (the "old" pics format used by earlier Legal Crime versions).
//
// Transparent pixels (A==0) are mapped to palette index 254 (ColorKeyIndex).
// Up to 253 unique opaque colors are supported; if the image contains more,
// the 253 most-frequent colors are kept and the rest are mapped to their
// nearest palette entry using Euclidean RGB distance.
func WriteOldFormat(path string, img *image.RGBA) error {
	palette, colorMap := quantizePalette(img)
	data := buildBMP8(img, palette, colorMap)
	return os.WriteFile(path, data, 0o644)
}

// WriteNewFormat writes img to path as a 24-bit true-color BMP
// XOR-obfuscated with key 0xCD (the "new" pics format used by later
// Legal Crime versions).
func WriteNewFormat(path string, img *image.RGBA) error {
	data := buildBMP24(img)
	// XOR-obfuscate the entire file with key 0xCD.
	for i := range data {
		data[i] ^= xorKey
	}
	return os.WriteFile(path, data, 0o644)
}

// DetectFormat returns "old" if the file is an unobfuscated 8-bit BMP,
// "new" if it is a 24-bit BMP XOR-obfuscated with 0xCD, or an error if
// neither pattern matches.
func DetectFormat(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	header := make([]byte, 30)
	if _, err := f.Read(header); err != nil {
		return "", fmt.Errorf("read header: %w", err)
	}

	// Old format: plain "BM" magic, bpp==8.
	if header[0] == 'B' && header[1] == 'M' {
		bpp := binary.LittleEndian.Uint16(header[28:30])
		if bpp == 8 {
			return "old", nil
		}
		if bpp == 24 {
			// Plain 24-bit BMP – treat as "new" without XOR for round-trip.
			return "old24", nil
		}
	}

	// New format: XOR'd BM magic, bpp==24 after decode.
	if header[0]^xorKey == 'B' && header[1]^xorKey == 'M' {
		bpp := binary.LittleEndian.Uint16([]byte{header[28] ^ xorKey, header[29] ^ xorKey})
		if bpp == 24 {
			return "new", nil
		}
	}

	return "", fmt.Errorf("unrecognised BMP format in %s", path)
}

// ─── internal helpers ────────────────────────────────────────────────────────

// packRGB packs opaque RGB values into a single uint32 key.
func packRGB(r, g, b uint8) uint32 {
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

// colorDist returns the squared Euclidean RGB distance between two colors.
func colorDist(a, b color.RGBA) int64 {
	dr := int64(a.R) - int64(b.R)
	dg := int64(a.G) - int64(b.G)
	db := int64(a.B) - int64(b.B)
	return dr*dr + dg*dg + db*db
}

// nearestIdx returns the palette index whose color is closest to c.
func nearestIdx(c color.RGBA, palette [256]color.RGBA, usedCount int) uint8 {
	best := uint8(0)
	bestDist := int64(math.MaxInt64)
	for i := 0; i < usedCount; i++ {
		if i == ColorKeyIndex {
			continue
		}
		d := colorDist(c, palette[i])
		if d < bestDist {
			bestDist = d
			best = uint8(i)
		}
	}
	return best
}

// quantizePalette builds a 256-entry palette for img and returns a
// per-pixel color → palette-index map.
//
// Transparent pixels (A==0) → index 254 (ColorKeyIndex).
// Opaque pixels: up to 253 most-frequent unique colors get their own slot;
// overflow colors map to the closest slot by Euclidean RGB distance.
func quantizePalette(img *image.RGBA) ([256]color.RGBA, map[uint32]uint8) {
	// Count frequency of each unique opaque color.
	freq := make(map[uint32]int)
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			freq[packRGB(c.R, c.G, c.B)]++
		}
	}

	// Sort by descending frequency, keep at most 253 entries.
	type kv struct {
		key   uint32
		count int
	}
	kvs := make([]kv, 0, len(freq))
	for k, v := range freq {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].count > kvs[j].count })
	if len(kvs) > 253 {
		kvs = kvs[:253]
	}

	// Assign palette indices. Skip index 254 (transparent).
	var palette [256]color.RGBA
	// Index 254 = transparent (leave zero = {0,0,0,0} = fully transparent).
	palette[ColorKeyIndex] = color.RGBA{}

	colorMap := make(map[uint32]uint8, len(kvs))
	idx := uint8(0)
	for _, kv := range kvs {
		if idx == ColorKeyIndex {
			idx++ // skip reserved transparent index
		}
		r := uint8(kv.key >> 16)
		g := uint8(kv.key >> 8)
		b := uint8(kv.key)
		palette[idx] = color.RGBA{R: r, G: g, B: b, A: 255}
		colorMap[kv.key] = idx
		idx++
	}
	usedCount := int(idx)

	// Pre-compute nearest-palette entries for any overflow colors.
	// (Only needed when the image has >253 unique opaque colors.)
	overflowCache := make(map[uint32]uint8)

	// Build a complete pixel→index map including overflows.
	// For overflow pixels we compute the nearest palette entry lazily.
	fullMap := make(map[uint32]uint8, len(freq))
	for k := range freq {
		if mi, ok := colorMap[k]; ok {
			fullMap[k] = mi
			continue
		}
		if mi, ok := overflowCache[k]; ok {
			fullMap[k] = mi
			continue
		}
		c := color.RGBA{R: uint8(k >> 16), G: uint8(k >> 8), B: uint8(k)}
		mi := nearestIdx(c, palette, usedCount)
		overflowCache[k] = mi
		fullMap[k] = mi
	}

	return palette, fullMap
}

// buildBMP8 serialises img as a raw uncompressed 8-bit BMP byte slice.
func buildBMP8(img *image.RGBA, palette [256]color.RGBA, colorMap map[uint32]uint8) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Row stride must be a multiple of 4.
	stride := (width + 3) &^ 3
	pixelDataSize := stride * height
	paletteSize := 256 * 4
	pixelOffset := 14 + 40 + paletteSize // 1078
	fileSize := pixelOffset + pixelDataSize

	buf := make([]byte, fileSize)

	// ── BMP file header (14 bytes) ──
	buf[0] = 'B'
	buf[1] = 'M'
	binary.LittleEndian.PutUint32(buf[2:], uint32(fileSize))
	// bytes 6-9: reserved, already zero
	binary.LittleEndian.PutUint32(buf[10:], uint32(pixelOffset))

	// ── BITMAPINFOHEADER (40 bytes, at offset 14) ──
	h := buf[14:]
	binary.LittleEndian.PutUint32(h[0:], 40) // DIB header size
	binary.LittleEndian.PutUint32(h[4:], uint32(width))
	binary.LittleEndian.PutUint32(h[8:], uint32(height))
	binary.LittleEndian.PutUint16(h[12:], 1) // color planes
	binary.LittleEndian.PutUint16(h[14:], 8) // bits per pixel
	// h[16..19]: compression = 0
	// h[20..23]: image size = 0 (can be 0 for uncompressed)
	// h[24..31]: resolution = 0
	binary.LittleEndian.PutUint32(h[32:], 256) // colors used
	binary.LittleEndian.PutUint32(h[36:], 256) // important colors

	// ── Palette (256 × 4 bytes, at offset 54) ──
	// RGBQUAD layout: Blue, Green, Red, Reserved.
	palOff := 54
	for i, c := range palette {
		off := palOff + i*4
		buf[off] = c.B
		buf[off+1] = c.G
		buf[off+2] = c.R
		buf[off+3] = 0
	}

	// ── Pixel data (bottom-up) ──
	for y := 0; y < height; y++ {
		srcY := bounds.Min.Y + (height - 1 - y) // bottom-up
		rowBase := pixelOffset + y*stride
		for x := 0; x < width; x++ {
			c := img.RGBAAt(bounds.Min.X+x, srcY)
			var pidx uint8
			if c.A == 0 {
				pidx = ColorKeyIndex
			} else {
				key := packRGB(c.R, c.G, c.B)
				pidx = colorMap[key]
			}
			buf[rowBase+x] = pidx
		}
	}

	return buf
}

// buildBMP24 serialises img as a raw uncompressed 24-bit true-color BMP
// byte slice (not yet XOR'd).
func buildBMP24(img *image.RGBA) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Row stride: 3 bytes per pixel, padded to 4-byte boundary.
	stride := (width*3 + 3) &^ 3
	pixelDataSize := stride * height
	pixelOffset := 54 // 14 + 40, no palette for 24-bit
	fileSize := pixelOffset + pixelDataSize

	buf := make([]byte, fileSize)

	// ── BMP file header (14 bytes) ──
	buf[0] = 'B'
	buf[1] = 'M'
	binary.LittleEndian.PutUint32(buf[2:], uint32(fileSize))
	// bytes 6-9: reserved, already zero
	binary.LittleEndian.PutUint32(buf[10:], uint32(pixelOffset))

	// ── BITMAPINFOHEADER (40 bytes, at offset 14) ──
	h := buf[14:]
	binary.LittleEndian.PutUint32(h[0:], 40) // DIB header size
	binary.LittleEndian.PutUint32(h[4:], uint32(width))
	binary.LittleEndian.PutUint32(h[8:], uint32(height))
	binary.LittleEndian.PutUint16(h[12:], 1)  // color planes
	binary.LittleEndian.PutUint16(h[14:], 24) // bits per pixel
	// h[16..53]: compression=0, imageSize=0, resolution=0, clrUsed=0, clrImportant=0 — already zero

	// ── Pixel data (bottom-up, BGR byte order) ──
	for y := 0; y < height; y++ {
		srcY := bounds.Min.Y + (height - 1 - y) // bottom-up
		rowBase := pixelOffset + y*stride
		for x := 0; x < width; x++ {
			c := img.RGBAAt(bounds.Min.X+x, srcY)
			off := rowBase + x*3
			buf[off] = c.B
			buf[off+1] = c.G
			buf[off+2] = c.R
		}
	}

	return buf
}
