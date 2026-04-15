package render

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"strings"

	"github.com/ldmonster/legal-crime-tools/internal/imageutil"
	"github.com/ldmonster/legal-crime-tools/internal/mapfile"
	"github.com/ldmonster/legal-crime-tools/internal/tile"
)

const (
	IsoHalfW = 20
	IsoHalfH = 10
	IsoCellH = 2 * IsoHalfH
)

// MapOptions controls which blocks are rendered.
type MapOptions struct {
	// FilterBT, if non-nil, limits rendering to only those block types.
	FilterBT map[int]bool
}

// BuildGrid expands map blocks into a flat tile-index grid.
// Returns grid[y*width+x] = tile index (-1 = empty when filtering).
func BuildGrid(m *mapfile.Def, opts *MapOptions) []int {
	fillDefault := -1
	if opts == nil || len(opts.FilterBT) == 0 {
		fillDefault = 0 // fill with Road tile when rendering full map
	}

	grid := make([]int, m.Width*m.Height)
	for i := range grid {
		grid[i] = fillDefault
	}

	for _, blk := range m.Blocks {
		if blk.BlockType < 0 || blk.BlockType >= len(tile.BlockTable) {
			fmt.Fprintf(os.Stderr, "map: unknown block type %d at (%d,%d)\n", blk.BlockType, blk.X, blk.Y)
			continue
		}
		if opts != nil && len(opts.FilterBT) > 0 && !opts.FilterBT[blk.BlockType] {
			continue
		}
		bi := tile.BlockTable[blk.BlockType]
		for dc := 0; dc < bi.Cols; dc++ {
			for dr := 0; dr < bi.Rows; dr++ {
				mx := blk.X + dc
				my := blk.Y + dr
				if mx < 0 || mx >= m.Width || my < 0 || my >= m.Height {
					continue
				}
				tileIdx := bi.BaseTile + dc*bi.Rows + (bi.Rows - 1 - dr)
				grid[my*m.Width+mx] = tileIdx
			}
		}
	}
	return grid
}

// Map composes an isometric map image, optionally filtering by block types.
func Map(
	m *mapfile.Def,
	tiles []tile.Def,
	images []string,
	cache *imageutil.BMPCache,
	opts *MapOptions,
) (*image.RGBA, error) {
	if m.Width <= 0 || m.Height <= 0 {
		return nil, fmt.Errorf("map has invalid dimensions %dx%d", m.Width, m.Height)
	}

	grid := BuildGrid(m, opts)

	maxTileH := IsoCellH
	for _, idx := range grid {
		if idx >= 0 && idx < len(tiles) {
			if h := tiles[idx].Height; h > maxTileH {
				maxTileH = h
			}
		}
	}

	topPad := maxTileH
	canvasW := (m.Width+m.Height)*IsoHalfW + 80
	canvasH := (m.Width+m.Height)*IsoHalfH + maxTileH + IsoHalfH
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))

	// Track drawn pixel bounds for auto-cropping when filtering.
	minPX, minPY := canvasW, canvasH
	maxPX, maxPY := 0, 0

	for d := 0; d < m.Width+m.Height-1; d++ {
		for col := 0; col < m.Width; col++ {
			row := d - col
			if row < 0 || row >= m.Height {
				continue
			}
			tileIdx := grid[row*m.Width+col]
			if tileIdx < 0 || tileIdx >= len(tiles) {
				continue
			}
			td := tiles[tileIdx]
			if td.ImgIndex < 0 || td.ImgIndex >= len(images) {
				continue
			}
			src, err := cache.Load(images[td.ImgIndex])
			if err != nil {
				continue
			}

			northX := (col - row + m.Height) * IsoHalfW
			northY := (col+row)*IsoHalfH + topPad
			drawX := northX - td.Width/2
			drawY := northY + IsoCellH - td.Height

			fx := td.FrameXs[0]
			fy := td.FrameYs[0]
			srcRect := image.Rect(fx, fy, fx+td.Width, fy+td.Height)
			dstRect := image.Rect(drawX, drawY, drawX+td.Width, drawY+td.Height)
			draw.Draw(canvas, dstRect, src, srcRect.Min, draw.Over)

			if drawX < minPX {
				minPX = drawX
			}
			if drawY < minPY {
				minPY = drawY
			}
			if drawX+td.Width > maxPX {
				maxPX = drawX + td.Width
			}
			if drawY+td.Height > maxPY {
				maxPY = drawY + td.Height
			}
		}
	}

	// When filtering by BT, crop canvas to just the drawn area.
	if opts != nil && len(opts.FilterBT) > 0 && maxPX > minPX && maxPY > minPY {
		cropRect := image.Rect(minPX, minPY, maxPX, maxPY)
		cropped := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
		draw.Draw(cropped, cropped.Bounds(), canvas, cropRect.Min, draw.Src)
		return cropped, nil
	}

	return canvas, nil
}

// Atlas saves a source BMP directly as a transparent PNG.
func Atlas(src *image.RGBA, outPath string) error {
	return imageutil.SavePNG(outPath, src)
}

// Block renders a single block type as a standalone isometric image.
// It uses the block's own Cols×Rows grid with tiles from the BlockTable,
// no map file needed.
func Block(
	bt int,
	tiles []tile.Def,
	images []string,
	cache *imageutil.BMPCache,
) (*image.RGBA, error) {
	if bt < 0 || bt >= len(tile.BlockTable) {
		return nil, fmt.Errorf("block type %d out of range (0-%d)", bt, len(tile.BlockTable)-1)
	}
	bi := tile.BlockTable[bt]
	cols, rows := bi.Cols, bi.Rows

	// Build tile index grid for this single block.
	// tile = BaseTile + dc*Rows + (Rows-1-dr)
	grid := make([]int, cols*rows)
	maxTileH := IsoCellH
	for dc := 0; dc < cols; dc++ {
		for dr := 0; dr < rows; dr++ {
			idx := bi.BaseTile + dc*rows + (rows - 1 - dr)
			grid[dr*cols+dc] = idx
			if idx >= 0 && idx < len(tiles) {
				if h := tiles[idx].Height; h > maxTileH {
					maxTileH = h
				}
			}
		}
	}

	topPad := maxTileH
	canvasW := (cols+rows)*IsoHalfW + 80
	canvasH := (cols+rows)*IsoHalfH + maxTileH + IsoHalfH
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))

	minPX, minPY := canvasW, canvasH
	maxPX, maxPY := 0, 0

	for d := 0; d < cols+rows-1; d++ {
		for col := 0; col < cols; col++ {
			row := d - col
			if row < 0 || row >= rows {
				continue
			}
			tileIdx := grid[row*cols+col]
			if tileIdx < 0 || tileIdx >= len(tiles) {
				continue
			}
			td := tiles[tileIdx]
			if td.ImgIndex < 0 || td.ImgIndex >= len(images) {
				continue
			}
			src, err := cache.Load(images[td.ImgIndex])
			if err != nil {
				continue
			}

			northX := (col - row + rows) * IsoHalfW
			northY := (col+row)*IsoHalfH + topPad
			drawX := northX - td.Width/2
			drawY := northY + IsoCellH - td.Height

			fx := td.FrameXs[0]
			fy := td.FrameYs[0]
			srcRect := image.Rect(fx, fy, fx+td.Width, fy+td.Height)
			dstRect := image.Rect(drawX, drawY, drawX+td.Width, drawY+td.Height)
			draw.Draw(canvas, dstRect, src, srcRect.Min, draw.Over)

			if drawX < minPX {
				minPX = drawX
			}
			if drawY < minPY {
				minPY = drawY
			}
			if drawX+td.Width > maxPX {
				maxPX = drawX + td.Width
			}
			if drawY+td.Height > maxPY {
				maxPY = drawY + td.Height
			}
		}
	}

	// Crop to drawn area.
	if maxPX > minPX && maxPY > minPY {
		cropRect := image.Rect(minPX, minPY, maxPX, maxPY)
		cropped := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
		draw.Draw(cropped, cropped.Bounds(), canvas, cropRect.Min, draw.Src)
		return cropped, nil
	}

	return canvas, nil
}

// Unified stacks all Block* source BMPs vertically into one PNG.
func Unified(images []string, cache *imageutil.BMPCache, outPath string) error {
	type entry struct {
		name string
		img  *image.RGBA
	}
	var sheets []entry
	maxW := 0
	totalH := 0
	for _, name := range images {
		if !strings.HasPrefix(strings.ToLower(name), "block") {
			continue
		}
		img, err := cache.Load(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unified: %s: %v\n", name, err)
			continue
		}
		sheets = append(sheets, entry{name, img})
		if img.Bounds().Dx() > maxW {
			maxW = img.Bounds().Dx()
		}
		totalH += img.Bounds().Dy()
	}
	if len(sheets) == 0 || maxW == 0 || totalH == 0 {
		return fmt.Errorf("no Block* images found")
	}

	unified := image.NewRGBA(image.Rect(0, 0, maxW, totalH))
	y := 0
	for _, s := range sheets {
		b := s.img.Bounds()
		draw.Draw(unified, image.Rect(0, y, b.Dx(), y+b.Dy()), s.img, image.Point{}, draw.Src)
		y += b.Dy()
	}
	return imageutil.SavePNG(outPath, unified)
}
