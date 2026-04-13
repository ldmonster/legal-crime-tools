package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ldmonster/legal-crime-tools/imageutil"
	"github.com/ldmonster/legal-crime-tools/mapfile"
	"github.com/ldmonster/legal-crime-tools/render"
	"github.com/ldmonster/legal-crime-tools/tile"
)

var (
	mapFilePath string
	mapOutPath  string
	mapDir      string
)

var renderMapCmd = &cobra.Command{
	Use:   "render-map",
	Short: "Render isometric map(s) from .map file(s)",
	Long: `Render a full isometric map. Without --map renders all .map files
found in --mapdir (default: current directory).

Example:
  github.com/ldmonster/legal-crime-tools render-map                      # render all .map files in .
  github.com/ldmonster/legal-crime-tools render-map --mapdir Maps         # render all .map files in Maps/
  github.com/ldmonster/legal-crime-tools render-map --map Chicago.map     # render one specific map`,
	RunE: runRenderMap,
}

func init() {
	renderMapCmd.Flags().StringVar(&mapFilePath, "map", "", "path to a single .map file (omit to render all)")
	renderMapCmd.Flags().StringVar(&mapOutPath, "mapout", "", "output PNG path (default: <out>/<mapname>_map.png)")
	renderMapCmd.Flags().StringVar(&mapDir, "mapdir", ".", "directory to scan for .map files when --map is omitted")
	rootCmd.AddCommand(renderMapCmd)
}

func runRenderMap(cmd *cobra.Command, args []string) error {
	tf, err := tile.ParseFile(tileFile)
	if err != nil {
		return fmt.Errorf("parse tile file: %w", err)
	}

	cache, err := buildCache()
	if err != nil {
		return err
	}

	if mapFilePath != "" {
		return renderOneMap(mapFilePath, tf, cache)
	}

	// render all .map files in mapDir
	matches, err := filepath.Glob(filepath.Join(mapDir, "*.map"))
	if err != nil {
		return fmt.Errorf("glob map files: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no .map files found in %s", mapDir)
	}
	for _, m := range matches {
		fmt.Printf("--- %s ---\n", m)
		if err := renderOneMap(m, tf, cache); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v (skipped)\n", m, err)
		}
	}
	return nil
}

func renderOneMap(path string, tf *tile.File, cache *imageutil.BMPCache) error {
	mapDef, err := mapfile.Parse(path)
	if err != nil {
		return fmt.Errorf("parse map file: %w", err)
	}
	fmt.Printf("Map: %dx%d grid, %d blocks\n", mapDef.Width, mapDef.Height, len(mapDef.Blocks))

	out := mapOutPath
	if out == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		out = filepath.Join(outDir, base+"_map.png")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	img, err := render.Map(mapDef, tf.Tiles, tf.Images, cache, nil)
	if err != nil {
		return fmt.Errorf("render map: %w", err)
	}
	if err := imageutil.SavePNG(out, img); err != nil {
		return fmt.Errorf("save map PNG: %w", err)
	}
	fmt.Printf("Map rendered -> %s\n", out)
	return nil
}
