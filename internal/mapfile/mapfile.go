// Package mapfile handles parsing of LC .map files.
package mapfile

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Block is one entry from the BLOCKS section of a .map file.
// X, Y are grid-cell coordinates; BlockType is the block type index (0-37).
type Block struct {
	X, Y      int
	BlockType int
}

// Def holds the parsed content of a .map file.
type Def struct {
	Width, Height int // XDIM, YDIM in grid cells
	Blocks        []Block
}

// Parse reads an LC .map file and extracts the grid dimensions and all
// explicit block placements from the BLOCKS section.
func Parse(path string) (*Def, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := &Def{}
	sc := bufio.NewScanner(f)
	inBlocks := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r\n ")
		if line == "" {
			continue
		}
		upper := strings.ToUpper(strings.TrimSpace(line))
		fields := strings.Fields(line)

		switch {
		case strings.HasPrefix(upper, "XDIM") && len(fields) >= 2:
			m.Width, _ = strconv.Atoi(fields[1])
		case strings.HasPrefix(upper, "YDIM") && len(fields) >= 2:
			m.Height, _ = strconv.Atoi(fields[1])
		case upper == "BLOCKS":
			inBlocks = true
		case upper == "STARTPOINTS" || upper == "PLAYERNAMES" || upper == "END":
			inBlocks = false
		case inBlocks && len(fields) >= 3:
			x, ex := strconv.Atoi(fields[0])
			y, ey := strconv.Atoi(fields[1])
			bt, et := strconv.Atoi(fields[2])
			if ex == nil && ey == nil && et == nil && x >= 0 && y >= 0 && bt >= 0 {
				m.Blocks = append(m.Blocks, Block{x, y, bt})
			}
		}
	}
	return m, sc.Err()
}
