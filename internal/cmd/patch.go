package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	patchRestore bool
)

// Original hardcoded server IPs in LegalCrime.EXE / Capone.EXE / Diary.EXE
// (all defunct). Offsets are discovered at runtime by scanning for these
// strings, so the same command works across binary variants (Capone ships
// them ~0x3200 earlier than LegalCrime).
type ipPatch struct {
	offset   int64
	original []byte
}

// originalIPs are the 4 defunct hardcoded IPs, each expected to be followed
// by a NUL terminator in the binary. Slot length = len(original)+1.
var originalIPs = []string{
	"208.194.67.16",  // 14-byte slot
	"194.100.92.102", // 15-byte slot
	"207.226.185.80", // 15-byte slot
	"194.100.92.99",  // 14-byte slot
}

// discoverPatches scans the binary for each original IP followed by NUL and
// returns a patch descriptor for each. All 4 must be found exactly once.
func discoverPatches(exePath string) ([]ipPatch, error) {
	data, err := os.ReadFile(exePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", exePath, err)
	}
	out := make([]ipPatch, 0, len(originalIPs))
	for _, ip := range originalIPs {
		needle := append([]byte(ip), 0x00)
		idx := bytes.Index(data, needle)
		if idx < 0 {
			return nil, fmt.Errorf("original IP %q not found in %s (binary already patched or unsupported variant? run --restore first)", ip, exePath)
		}
		if dup := bytes.Index(data[idx+1:], needle); dup >= 0 {
			return nil, fmt.Errorf("original IP %q found multiple times (ambiguous) in %s", ip, exePath)
		}
		out = append(out, ipPatch{offset: int64(idx), original: needle})
	}
	return out, nil
}

var patchCmd = &cobra.Command{
	Use:   "patch <EXE> <IP>",
	Short: "Patch LegalCrime.EXE to replace hardcoded dead server IPs",
	Long: `Patch LegalCrime.EXE to replace hardcoded dead server IPs with a custom IP.

Original IPs (all defunct):
  208.194.67.16    @ file offset 0x78c0a
  194.100.92.102   @ file offset 0x78c18
  207.226.185.80   @ file offset 0x78c27
  194.100.92.99    @ file offset 0x78c36

Examples:
  legal-crime-tools patch LegalCrime.EXE 192.168.1.100   # patch all 4 IPs
  legal-crime-tools patch LegalCrime.EXE --restore        # restore from backup`,
	RunE: runPatch,
}

func init() {
	patchCmd.Flags().BoolVar(&patchRestore, "restore", false, "restore EXE from .orig backup")
	rootCmd.AddCommand(patchCmd)
}

func runPatch(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("EXE path is required")
	}
	exePath := args[0]

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", exePath)
	}

	if patchRestore {
		return restoreEXE(exePath)
	}

	if len(args) < 2 {
		return fmt.Errorf("IP address is required (or use --restore)")
	}
	return patchEXE(exePath, args[1])
}

func maxIPLen() int {
	minSlot := len(originalIPs[0])
	for _, ip := range originalIPs[1:] {
		if sl := len(ip); sl < minSlot {
			minSlot = sl
		}
	}
	return minSlot
}

func patchEXE(exePath, ip string) error {
	ipBytes := []byte(ip)
	maxLen := maxIPLen()
	if len(ipBytes) > maxLen {
		return fmt.Errorf("IP '%s' is %d chars, max is %d", ip, len(ipBytes), maxLen)
	}

	patches, err := discoverPatches(exePath)
	if err != nil {
		return err
	}
	fmt.Printf("Discovered %d IP slots:\n", len(patches))
	for _, p := range patches {
		fmt.Printf("  0x%05x: %s\n", p.offset, nullTermString(p.original))
	}

	backup := exePath + ".orig"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		src, err := os.ReadFile(exePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", exePath, err)
		}
		if err := os.WriteFile(backup, src, 0644); err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
		fmt.Printf("Backup saved to: %s\n", backup)
	} else {
		fmt.Printf("Backup already exists: %s\n", backup)
	}

	f, err := os.OpenFile(exePath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", exePath, err)
	}
	defer f.Close()

	for _, p := range patches {
		slotLen := len(p.original)
		padded := make([]byte, slotLen)
		copy(padded, ipBytes)
		// Remaining bytes are already zero-valued from make()

		data := make([]byte, slotLen)
		if _, err := f.ReadAt(data, p.offset); err != nil {
			return fmt.Errorf("read at 0x%05x: %w", p.offset, err)
		}

		if bytesEqual(data, padded) {
			fmt.Printf("  0x%05x: already patched to %s\n", p.offset, ip)
			continue
		}

		// Accept either original bytes or any previously-patched value (null-terminated)
		if !bytesEqual(data, p.original) && data[len(data)-1] != 0x00 {
			fmt.Printf("  0x%05x: UNEXPECTED DATA: %q\n", p.offset, data)
			return fmt.Errorf("unexpected data at 0x%05x — restore from backup first (--restore)", p.offset)
		}

		if _, err := f.WriteAt(padded, p.offset); err != nil {
			return fmt.Errorf("write at 0x%05x: %w", p.offset, err)
		}

		oldStr := nullTermString(data)
		fmt.Printf("  0x%05x: %s -> %s\n", p.offset, oldStr, ip)
	}

	fmt.Printf("\nDone! All server IPs patched to %s\n", ip)
	return nil
}

func restoreEXE(exePath string) error {
	backup := exePath + ".orig"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		return fmt.Errorf("no backup found (%s), cannot restore", backup)
	}

	src, err := os.ReadFile(backup)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}
	if err := os.WriteFile(exePath, src, 0644); err != nil {
		return fmt.Errorf("write %s: %w", exePath, err)
	}
	fmt.Printf("Restored from %s\n", backup)
	return nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func nullTermString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
