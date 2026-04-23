package cmd

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ldmonster/legal-crime-tools/internal/imageutil"
)

var (
	animateInput  string
	animateOutput string
	animateSide   int
)

// frameRe matches the "_fNN" frame suffix (optionally followed by a duplicate
// index "_K") produced by the extract command on multi-frame tiles.
//
// Examples:
//
//	Enemy_Godfather_NE_f00.png      → base="Enemy_Godfather_NE" idx=0  dup=""
//	Enemy_bat_man_idle_f03.png      → base="Enemy_bat_man_idle" idx=3  dup=""
//	Walker_W_f02_1.png              → base="Walker_W"           idx=2  dup="1"
var frameRe = regexp.MustCompile(`^(.+)_f(\d+)(?:_(\d+))?\.png$`)

var animateCmd = &cobra.Command{
	Use:   "animate",
	Short: "Assemble extracted tile frames into horizontal animation strips",
	Long: `Scan --input for PNG tiles named "<base>_fNN.png" (as emitted by the
extract command), group them by base name, and write one horizontal strip per
animation into --output.

Each frame is pasted (no scaling) into the centre of a --side × --side cell;
strip width = frames × side, height = side.  Frames larger than --side are
clipped.  PNGs without a "_fNN" suffix are ignored (they are not animations).`,
	RunE: runAnimate,
}

func init() {
	animateCmd.Flags().StringVarP(&animateInput, "input", "i", "", "directory containing extracted tile PNGs (required)")
	animateCmd.Flags().StringVarP(&animateOutput, "output", "o", "animations", "output directory for animation strips")
	animateCmd.Flags().IntVar(&animateSide, "side", 0, "side length in pixels of each animation frame cell (required)")
	_ = animateCmd.MarkFlagRequired("input")
	_ = animateCmd.MarkFlagRequired("side")
	rootCmd.AddCommand(animateCmd)
}

// animKey uniquely identifies one animation: subdir + base name + optional
// duplicate-group index.  Frames extracted from duplicate tile definitions
// (suffix "_K") form separate animations so they are not mixed together.
type animKey struct {
	subdir string
	base   string
	dup    string // "" for the primary group; "1", "2", … for duplicates
}

// animFrame represents a single frame PNG belonging to an animation.
type animFrame struct {
	idx  int
	path string
}

func runAnimate(cmd *cobra.Command, args []string) error {
	if animateSide <= 0 {
		return fmt.Errorf("--side must be a positive integer, got %d", animateSide)
	}

	info, err := os.Stat(animateInput)
	if err != nil {
		return fmt.Errorf("stat input: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("--input must be a directory")
	}

	// Collect every *_fNN*.png file grouped by animation.
	groups := make(map[animKey][]animFrame)
	err = filepath.WalkDir(animateInput, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.EqualFold(filepath.Ext(name), ".png") {
			return nil
		}
		m := frameRe.FindStringSubmatch(name)
		if m == nil {
			return nil
		}
		idx, err := strconv.Atoi(m[2])
		if err != nil {
			return nil
		}
		base, dup := m[1], m[3]

		rel, err := filepath.Rel(animateInput, filepath.Dir(path))
		if err != nil {
			rel = ""
		}
		if rel == "." {
			rel = ""
		}
		key := animKey{subdir: rel, base: base, dup: dup}
		groups[key] = append(groups[key], animFrame{idx: idx, path: path})
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk input: %w", err)
	}

	if len(groups) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no animation frames (*_fNN.png) found under input")
		return nil
	}

	if err := os.MkdirAll(animateOutput, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Iterate in deterministic order (stable output regardless of FS walk).
	keys := make([]animKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].subdir != keys[j].subdir {
			return keys[i].subdir < keys[j].subdir
		}
		if keys[i].base != keys[j].base {
			return keys[i].base < keys[j].base
		}
		return keys[i].dup < keys[j].dup
	})

	var written, skipped int
	for _, k := range keys {
		frames := groups[k]
		// Sort frames by their fNN index.
		sort.Slice(frames, func(i, j int) bool { return frames[i].idx < frames[j].idx })

		outName := k.base + ".png"
		if k.dup != "" {
			outName = fmt.Sprintf("%s_%s.png", k.base, k.dup)
		}
		outPath := filepath.Join(animateOutput, k.subdir, outName)

		if err := composeStrip(frames, animateSide, outPath); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: %v\n", outPath, err)
			skipped++
			continue
		}
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d frames)\n", outPath, len(frames))
		}
		written++
	}

	fmt.Fprintf(cmd.OutOrStdout(), "done: %d animations written, %d skipped -> %s/\n",
		written, skipped, animateOutput)
	return nil
}

// composeStrip builds a horizontal strip of len(frames) cells, each side×side
// pixels, pastes each frame centred inside its cell, and writes the result
// as a PNG to outPath.
func composeStrip(frames []animFrame, side int, outPath string) error {
	if len(frames) == 0 {
		return fmt.Errorf("no frames")
	}

	stripW := side * len(frames)
	strip := image.NewRGBA(image.Rect(0, 0, stripW, side))

	for i, f := range frames {
		src, err := loadPNG(f.path)
		if err != nil {
			return fmt.Errorf("load %s: %w", f.path, err)
		}
		sb := src.Bounds()
		// Centre-position the frame inside its side×side cell.
		dx := i*side + (side-sb.Dx())/2
		dy := (side - sb.Dy()) / 2
		dstRect := image.Rect(dx, dy, dx+sb.Dx(), dy+sb.Dy())
		draw.Draw(strip, dstRect, src, sb.Min, draw.Src)
	}

	return imageutil.SavePNG(outPath, strip)
}

// loadPNG reads a PNG file and returns it as *image.RGBA.
func loadPNG(path string) (*image.RGBA, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	img, err := png.Decode(fh)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba, nil
}
