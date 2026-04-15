// Package cmd implements the Cobra CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ldmonster/legal-crime-tools/internal/imageutil"
)

var (
	tileFile string
	picsDir  string
	outDir   string
	verbose  bool
)

var rootCmd = &cobra.Command{
	Use:   "github.com/ldmonster/legal-crime-tools",
	Short: "Extract tile sprites and render isometric map views from LC game data",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&tileFile, "tile", "Chicago.tile", "path to .tile descriptor file")
	rootCmd.PersistentFlags().StringVar(&picsDir, "pics", "Pics", "directory containing BMP sprite sheets")
	rootCmd.PersistentFlags().StringVar(&outDir, "out", "tiles_out", "output directory")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print progress")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// buildCache creates a BMPCache from --pics and optionally --capone.
func buildCache() (*imageutil.BMPCache, error) {
	idx, err := imageutil.BuildPicsIndex(picsDir)
	if err != nil {
		return nil, fmt.Errorf("read pics dir: %w", err)
	}
	cache := imageutil.NewBMPCache(picsDir, idx)

	return cache, nil
}
