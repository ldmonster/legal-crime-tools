package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ldmonster/legal-crime-tools/internal/bmp"
)

var (
	convertInput  string
	convertOutput string
	convertTo     string
)

var convertPicsCmd = &cobra.Command{
	Use:   "convert-pics",
	Short: "Convert BMP pics between old (8-bit) and new (24-bit XOR) formats",
	Long: `Convert BMP sprite sheets between the two Legal Crime pics formats:

  old  – unobfuscated 8-bit indexed BMP (used by older game versions)
  new  – 24-bit true-color BMP XOR-obfuscated with key 0xCD (newer versions)

If --input is a directory every *.bmp / *.BMP file inside it is converted
and the results are written to --output (which must also be a directory, or
will be created).  If --input is a single file, --output should be a file path.

The format of the input file is detected automatically; --to specifies the
desired output format ("old" or "new").`,
	RunE: runConvertPics,
}

func init() {
	convertPicsCmd.Flags().StringVarP(&convertInput, "input", "i", "", "input BMP file or directory (required)")
	convertPicsCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "output BMP file or directory (required)")
	convertPicsCmd.Flags().StringVar(&convertTo, "to", "", `target format: "old" (8-bit) or "new" (24-bit XOR) (required)`)
	_ = convertPicsCmd.MarkFlagRequired("input")
	_ = convertPicsCmd.MarkFlagRequired("output")
	_ = convertPicsCmd.MarkFlagRequired("to")
	rootCmd.AddCommand(convertPicsCmd)
}

func runConvertPics(cmd *cobra.Command, args []string) error {
	convertTo = strings.ToLower(strings.TrimSpace(convertTo))
	if convertTo != "old" && convertTo != "new" {
		return fmt.Errorf("--to must be \"old\" or \"new\", got %q", convertTo)
	}

	info, err := os.Stat(convertInput)
	if err != nil {
		return fmt.Errorf("stat input: %w", err)
	}

	if info.IsDir() {
		return convertDirectory(cmd, convertInput, convertOutput, convertTo)
	}
	return convertSingleFile(cmd, convertInput, convertOutput, convertTo)
}

// convertDirectory converts all BMP files in srcDir, writing results to dstDir.
func convertDirectory(cmd *cobra.Command, srcDir, dstDir, toFmt string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", srcDir, err)
	}

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", dstDir, err)
	}

	var converted, skipped int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".bmp") {
			continue
		}

		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(dstDir, name)

		if err := convertSingleFile(cmd, srcPath, dstPath, toFmt); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: %v\n", name, err)
			skipped++
			continue
		}
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "converted %s\n", name)
		}
		converted++
	}

	fmt.Fprintf(cmd.OutOrStdout(), "done: %d converted, %d skipped\n", converted, skipped)
	return nil
}

// convertSingleFile converts one BMP file from its detected format to toFmt.
func convertSingleFile(cmd *cobra.Command, srcPath, dstPath, toFmt string) error {
	detectedFmt, err := bmp.DetectFormat(srcPath)
	if err != nil {
		return fmt.Errorf("detect format of %s: %w", srcPath, err)
	}

	// Normalise "old24" (plain 24-bit) → treat as "new" without XOR for
	// reading purposes; the writer will apply the correct transform.
	if detectedFmt == toFmt || (detectedFmt == "old24" && toFmt == "new") {
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: already in %q format, skipping\n", filepath.Base(srcPath), toFmt)
		}
		return nil
	}

	img, err := bmp.Read(srcPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", srcPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	switch toFmt {
	case "old":
		return bmp.WriteOldFormat(dstPath, img)
	case "new":
		return bmp.WriteNewFormat(dstPath, img)
	default:
		return fmt.Errorf("unknown target format %q", toFmt)
	}
}
