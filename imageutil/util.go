package imageutil

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ldmonster/legal-crime-tools/bmp"
)

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// SanitizeName turns a free-form comment into a safe filename stem.
func SanitizeName(s string) string {
	s = strings.TrimSuffix(s, " *")
	s = strings.TrimSuffix(s, ".bmp")
	s = strings.TrimSuffix(s, ".BMP")
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, s)
	s = nonAlnum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}

// CropRGBA extracts a sub-rectangle from src into a new RGBA image.
func CropRGBA(src *image.RGBA, rect image.Rectangle) *image.RGBA {
	bounds := src.Bounds()
	if !rect.Overlaps(bounds) {
		return image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	}
	clipped := rect.Intersect(bounds)
	dst := image.NewRGBA(image.Rect(0, 0, clipped.Dx(), clipped.Dy()))
	draw.Draw(dst, dst.Bounds(), src, clipped.Min, draw.Src)
	return dst
}

// SavePNG writes img to path as a PNG, creating parent directories as needed.
func SavePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	return png.Encode(fh, img)
}

// PicsIndex is a case-insensitive filename to realname map for a directory
// (needed because Windows paths in .tile don't match Linux filenames).
type PicsIndex map[string]string

// BuildPicsIndex creates a case-insensitive filename index for the given directory.
func BuildPicsIndex(dir string) (PicsIndex, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	idx := make(PicsIndex, len(entries))
	for _, e := range entries {
		idx[strings.ToLower(e.Name())] = e.Name()
	}
	return idx, nil
}

// Resolve finds the real filename case-insensitively.
func (idx PicsIndex) Resolve(name string) (string, bool) {
	real, ok := idx[strings.ToLower(name)]
	return real, ok
}

// picsSource is one directory + its case-insensitive index.
type picsSource struct {
	dir string
	idx PicsIndex
}

// BMPCache is a cache of decoded BMP images keyed by lowercase filename.
// Supports multiple pic directories searched in order.
type BMPCache struct {
	cache   map[string]*image.RGBA
	sources []picsSource
}

// NewBMPCache creates a new BMP cache backed by the given pics directory.
func NewBMPCache(picsDir string, idx PicsIndex) *BMPCache {
	return &BMPCache{
		cache:   make(map[string]*image.RGBA),
		sources: []picsSource{{dir: picsDir, idx: idx}},
	}
}

// AddSource adds a fallback pics directory to search.
func (c *BMPCache) AddSource(dir string, idx PicsIndex) {
	c.sources = append(c.sources, picsSource{dir: dir, idx: idx})
}

// Load returns a decoded BMP by its tile-file image name, using cache.
// Searches all sources in order.
func (c *BMPCache) Load(name string) (*image.RGBA, error) {
	key := strings.ToLower(name)
	if img, ok := c.cache[key]; ok {
		return img, nil
	}
	for _, src := range c.sources {
		real, ok := src.idx.Resolve(name)
		if !ok {
			continue
		}
		img, err := bmp.Read(filepath.Join(src.dir, real))
		if err != nil {
			continue
		}
		c.cache[key] = img
		return img, nil
	}
	return nil, os.ErrNotExist
}
