package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ldmonster/legal-crime-tools/imageutil"
	"github.com/ldmonster/legal-crime-tools/render"
	"github.com/ldmonster/legal-crime-tools/tile"
)

var (
	btOutPath string
	btType    int
)

var renderBTCmd = &cobra.Command{
	Use:   "render-bt",
	Short: "Render block type(s) (BT) as standalone isometric images",
	Long: `Render block types in isolation using their own Cols×Rows tile grids.
No map file needed. Without --bt renders all BTs (0-37).

Example:
  github.com/ldmonster/legal-crime-tools render-bt            # render all block types
  github.com/ldmonster/legal-crime-tools render-bt --bt 0     # render only BT 0
  github.com/ldmonster/legal-crime-tools render-bt --bt 18 --btout bt_renders/bt18.png`,
	RunE: runRenderBT,
}

func init() {
	renderBTCmd.Flags().IntVar(&btType, "bt", -1, "block type index to render (0-37); omit to render all")
	renderBTCmd.Flags().StringVar(&btOutPath, "btout", "", "output PNG path (default: <out>/bt_<N>.png)")
	rootCmd.AddCommand(renderBTCmd)
}

func runRenderBT(cmd *cobra.Command, args []string) error {
	tf, err := tile.ParseFile(tileFile)
	if err != nil {
		return fmt.Errorf("parse tile file: %w", err)
	}

	cache, err := buildCache()
	if err != nil {
		return err
	}

	if btType >= 0 {
		// render single BT
		if btType >= len(tile.BlockTable) {
			return fmt.Errorf("block type %d is out of range (0-%d)", btType, len(tile.BlockTable)-1)
		}
		return renderOneBT(btType, tf, cache)
	}

	// render all BTs
	for i := 0; i < len(tile.BlockTable); i++ {
		if err := renderOneBT(i, tf, cache); err != nil {
			fmt.Fprintf(os.Stderr, "BT %d: %v (skipped)\n", i, err)
		}
	}
	return nil
}

func renderOneBT(bt int, tf *tile.File, cache *imageutil.BMPCache) error {
	bi := tile.BlockTable[bt]
	fmt.Printf("BT %d: %dx%d grid, base tile %d\n", bt, bi.Cols, bi.Rows, bi.BaseTile)

	out := btOutPath
	if out == "" {
		out = filepath.Join(outDir, fmt.Sprintf("bt_%02d.png", bt))
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	img, err := render.Block(bt, tf.Tiles, tf.Images, cache)
	if err != nil {
		return fmt.Errorf("render BT: %w", err)
	}
	if err := imageutil.SavePNG(out, img); err != nil {
		return fmt.Errorf("save BT PNG: %w", err)
	}
	fmt.Printf("BT %d rendered -> %s (%dx%d px)\n", bt, out, img.Bounds().Dx(), img.Bounds().Dy())
	return nil
}
