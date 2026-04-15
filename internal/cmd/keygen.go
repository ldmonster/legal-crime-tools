package cmd

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	keygenCount  int
	keygenWrite  string
	keygenVerify string
)

type licenseKey struct {
	num0 int64
	nums [5]int64
	cs   int64
}

func keyChecksum(nums [5]int64) int64 {
	var cs int64
	for i, v := range nums {
		if i%2 == 0 {
			cs += v * 10
		} else {
			cs += v
		}
	}
	return cs % 12345
}

func keyRandRange(lo, hi int64) (int64, error) {
	diff := big.NewInt(hi - lo + 1)
	n, err := rand.Int(rand.Reader, diff)
	if err != nil {
		return 0, fmt.Errorf("crypto/rand: %w", err)
	}
	return n.Int64() + lo, nil
}

func generateKey() (licenseKey, error) {
	k := licenseKey{}
	var err error
	k.num0, err = keyRandRange(2500, 99999)
	if err != nil {
		return k, err
	}
	for i := range k.nums {
		k.nums[i], err = keyRandRange(10000, 200000)
		if err != nil {
			return k, err
		}
	}
	k.cs = keyChecksum(k.nums)
	return k, nil
}

func (k licenseKey) String() string {
	return fmt.Sprintf("%d %d %d %d %d %d %d",
		k.num0, k.nums[0], k.nums[1], k.nums[2], k.nums[3], k.nums[4], k.cs)
}

func (k licenseKey) Verify() bool {
	return k.num0 > 2499 && k.cs == keyChecksum(k.nums)
}

func parseKey(line string) (licenseKey, error) {
	line = strings.TrimSpace(line)
	parts := strings.Fields(line)
	if len(parts) < 7 {
		return licenseKey{}, fmt.Errorf("expected 7 numbers, got %d", len(parts))
	}
	vals := make([]int64, 7)
	for i := 0; i < 7; i++ {
		v, err := strconv.ParseInt(parts[i], 10, 64)
		if err != nil {
			return licenseKey{}, fmt.Errorf("field %d: %w", i, err)
		}
		vals[i] = v
	}
	k := licenseKey{num0: vals[0], cs: vals[6]}
	copy(k.nums[:], vals[1:6])
	return k, nil
}

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate or verify LCData.bin license keys",
	Long: `Generate license keys for Legal Crime, write them to LCData.bin, or verify existing key files.

Examples:
  legal-crime-tools keygen                        # generate key
  legal-crime-tools keygen -n 5                   # generate 5 keys
  legal-crime-tools keygen --write LCData.bin     # write a single key to file
  legal-crime-tools keygen --verify LCData.bin    # verify an existing key file`,
	RunE: runKeygen,
}

func init() {
	keygenCmd.Flags().IntVarP(&keygenCount, "count", "n", 1, "number of keys to generate")
	keygenCmd.Flags().StringVar(&keygenWrite, "write", "", "write a single key to LCData.bin at this path")
	keygenCmd.Flags().StringVar(&keygenVerify, "verify", "", "verify an existing LCData.bin file")
	rootCmd.AddCommand(keygenCmd)
}

func runKeygen(cmd *cobra.Command, args []string) error {
	if keygenVerify != "" {
		data, err := os.ReadFile(keygenVerify)
		if err != nil {
			return fmt.Errorf("read %s: %w", keygenVerify, err)
		}
		k, err := parseKey(string(data))
		if err != nil {
			return fmt.Errorf("parse error: %w", err)
		}
		expected := keyChecksum(k.nums)
		if k.Verify() {
			fmt.Printf("VALID: %s\n", k)
			fmt.Printf("  num0=%d (>2499), checksum=%d (match)\n", k.num0, k.cs)
		} else {
			fmt.Printf("INVALID: %s\n", k)
			if k.num0 <= 2499 {
				fmt.Printf("  num0=%d (must be >2499)\n", k.num0)
			}
			if k.cs != expected {
				fmt.Printf("  checksum=%d, expected=%d\n", k.cs, expected)
			}
		}
		return nil
	}

	if keygenWrite != "" {
		k, err := generateKey()
		if err != nil {
			return err
		}
		content := k.String() + "\r\n"
		if err := os.WriteFile(keygenWrite, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", keygenWrite, err)
		}
		fmt.Printf("Wrote key to %s: %s\n", keygenWrite, k)
		return nil
	}

	for i := 1; i <= keygenCount; i++ {
		k, err := generateKey()
		if err != nil {
			return err
		}
		fmt.Printf("%2d. %s\n", i, k)
	}
	return nil
}
