package cmd

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ldmonster/legal-crime-tools/internal/imageutil"
	"github.com/ldmonster/legal-crime-tools/internal/tile"
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract individual tile sprites from sprite sheets as PNG files",
	RunE:  runExtract,
}

func init() {
	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
	tf, err := tile.ParseFile(tileFile)
	if err != nil {
		return fmt.Errorf("parse tile file: %w", err)
	}
	fmt.Printf("Parsed %d images, %d tile definitions\n", len(tf.Images), len(tf.Tiles))

	cache, err := buildCache()
	if err != nil {
		return err
	}

	usedNames := make(map[string]int)
	skipped := 0
	extracted := 0

	for tileIdx, td := range tf.Tiles {
		if td.ImgIndex < 0 || td.ImgIndex >= len(tf.Images) {
			fmt.Fprintf(cmd.ErrOrStderr(), "tile %d: img_index %d out of range\n", tileIdx, td.ImgIndex)
			skipped++
			continue
		}

		imgName := tf.Images[td.ImgIndex]
		src, err := cache.Load(imgName)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "tile %d (%s): %v\n", tileIdx, imgName, err)
			skipped++
			continue
		}

		subdir := imageutil.SanitizeName(strings.TrimSuffix(imgName, filepath.Ext(imgName)))
		baseName := imageutil.SanitizeName(td.Comment)
		if baseName == "" {
			baseName = fmt.Sprintf("tile_%04d", tileIdx)
		}

		for frameIdx := range td.FrameXs {
			fx := td.FrameXs[frameIdx]
			fy := td.FrameYs[frameIdx]

			rect := image.Rect(fx, fy, fx+td.Width, fy+td.Height)
			frame := imageutil.CropRGBA(src, rect)

			frameSuffix := ""
			if len(td.FrameXs) > 1 {
				frameSuffix = fmt.Sprintf("_f%02d", frameIdx)
			}

			nameKey := filepath.Join(subdir, baseName+frameSuffix)
			count := usedNames[nameKey]
			usedNames[nameKey] = count + 1

			var outName string
			if count == 0 {
				outName = baseName + frameSuffix + ".png"
			} else {
				outName = fmt.Sprintf("%s%s_%d.png", baseName, frameSuffix, count)
			}

			outPath := filepath.Join(outDir, subdir, outName)
			if err := imageutil.SavePNG(outPath, frame); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "tile %d frame %d: save failed: %v\n", tileIdx, frameIdx, err)
				skipped++
				continue
			}

			if verbose {
				fmt.Printf("  [%04d/%d] %s -> %s\n", tileIdx, frameIdx, td.Comment, outPath)
			}
			extracted++
		}
	}

	fmt.Printf("Done: %d frames extracted, %d skipped -> %s/\n", extracted, skipped, outDir)
	return nil
}
