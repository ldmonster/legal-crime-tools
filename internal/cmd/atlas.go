package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ldmonster/legal-crime-tools/internal/imageutil"
	"github.com/ldmonster/legal-crime-tools/internal/render"
	"github.com/ldmonster/legal-crime-tools/internal/tile"
)

var atlasCmd = &cobra.Command{
	Use:   "atlas",
	Short: "Compose full-block atlas PNGs per source image and a unified block sheet",
	RunE:  runAtlas,
}

func init() {
	rootCmd.AddCommand(atlasCmd)
}

func runAtlas(cmd *cobra.Command, args []string) error {
	tf, err := tile.ParseFile(tileFile)
	if err != nil {
		return fmt.Errorf("parse tile file: %w", err)
	}

	cache, err := buildCache()
	if err != nil {
		return err
	}

	atlasDir := filepath.Join(outDir, "_atlas")
	fmt.Printf("Composing block atlases -> %s/\n", atlasDir)
	if err := os.MkdirAll(atlasDir, 0o755); err != nil {
		return fmt.Errorf("create atlas dir: %w", err)
	}

	for imgIdx, imgName := range tf.Images {
		src, err := cache.Load(imgName)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "atlas %s: %v\n", imgName, err)
			continue
		}
		baseName := imageutil.SanitizeName(strings.TrimSuffix(imgName, filepath.Ext(imgName)))
		atlasPath := filepath.Join(atlasDir, baseName+".png")
		if err := render.Atlas(src, atlasPath); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "atlas %s: %v\n", imgName, err)
		} else {
			fmt.Printf("  [%02d] %s -> %s\n", imgIdx, imgName, atlasPath)
		}
	}

	unifiedPath := filepath.Join(atlasDir, "blocks_unified.png")
	if err := render.Unified(tf.Images, cache, unifiedPath); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "unified: %v\n", err)
	} else {
		fmt.Printf("  unified blocks -> %s\n", unifiedPath)
	}

	return nil
}
