// Package tile handles .tile descriptor file parsing and block type definitions.
package tile

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Def describes a single tile sprite within a sprite sheet.
type Def struct {
	ImgIndex int
	Width    int
	Height   int
	FrameXs  []int
	FrameYs  []int
	Comment  string
}

// BlockInfo describes a block type's tile layout.
// Cols/Rows are the game's GetCols/GetRows for this block type.
// BaseTile is the starting tile index in the tile file.
// Tile index formula: BaseTile + dc*Rows + (Rows - 1 - dr)
//
//	where dc ∈ [0,Cols), dr ∈ [0,Rows).
type BlockInfo struct {
	Cols, Rows int
	BaseTile   int
}

// BlockTable maps blockType (0-37) → BlockInfo.
// Derived empirically from Chicago.tile structure and lc.c MapRoom_PosFromCoord.
var BlockTable = [38]BlockInfo{
	// BT  0-7:  12×12 interleaved CH1+CH{N+2}, stride 144
	0: {12, 12, 74},
	1: {12, 12, 218},
	2: {12, 12, 362},
	3: {12, 12, 506},
	4: {12, 12, 650},
	5: {12, 12, 794},
	6: {12, 12, 938},
	7: {12, 12, 1082},
	// BT  8-11: 12×12 sequential (CH9, CH12, CH11, CH10)
	8:  {12, 12, 1226},
	9:  {12, 12, 1370},
	10: {12, 12, 1514},
	11: {12, 12, 1658},
	// BT 12-13: 12 game-cols × 8 game-rows (CH13, CH14: 12 file-rows, 8 file-cols)
	12: {12, 8, 1810},
	13: {12, 8, 1906},
	// BT 14-15: 8 game-cols × 12 game-rows (CH15, CH16: 8 file-rows, 12 file-cols)
	14: {8, 12, 2002},
	15: {8, 12, 2098},
	// BT 16-17: 12×12 interleaved (CH1+CH17, CH1+CH3'/CH18)
	16: {12, 12, 2194},
	17: {12, 12, 2338},
	// BT 18-19: 4×4 composite (Park+CH mix)
	18: {4, 4, 3107},
	19: {4, 4, 3123},
	// BT 20-23: 8×8 composite
	20: {8, 8, 3139},
	21: {8, 8, 3203},
	22: {8, 8, 3267},
	23: {8, 8, 3331},
	// BT 24-33: small CH blocks
	24: {5, 5, 2809},
	25: {5, 5, 2834},
	26: {3, 4, 2859},
	27: {3, 3, 2871},
	28: {3, 3, 2880},
	29: {4, 3, 2889},
	30: {3, 3, 2901},
	31: {4, 3, 2910},
	32: {5, 5, 2922},
	33: {12, 12, 2947},
	// BT 34: 1×1 road/marker block
	34: {1, 1, 0},
	// BT 35-37: 4×4 park blocks
	35: {4, 4, 3411},
	36: {4, 4, 3429},
	37: {4, 4, 3445},
}

// File holds the parsed content of a .tile descriptor file.
type File struct {
	Images []string // ordered image filenames
	Tiles  []Def    // tile definitions
}

// ParseFile reads a Chicago.tile-style descriptor file.
func ParseFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	readLine := func() (string, bool) {
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r\n ")
			if line != "" {
				return line, true
			}
		}
		return "", false
	}
	parseInt := func(s string) int {
		v, _ := strconv.Atoi(strings.TrimSpace(s))
		return v
	}

	line, ok := readLine()
	if !ok {
		return nil, fmt.Errorf("tile file is empty")
	}
	nImages := parseInt(line)

	images := make([]string, 0, nImages)
	for i := 0; i < nImages; i++ {
		line, ok = readLine()
		if !ok {
			return nil, fmt.Errorf("expected %d image names, got %d", nImages, i)
		}
		name := strings.Fields(line)[0]
		images = append(images, name)
	}

	line, ok = readLine()
	if !ok {
		return nil, fmt.Errorf("expected tile count after image list")
	}
	nTiles := parseInt(line)

	tiles := make([]Def, 0, nTiles)
	for i := 0; i < nTiles; i++ {
		line, ok = readLine()
		if !ok {
			break
		}

		comment := ""
		if idx := strings.Index(line, ";"); idx >= 0 {
			comment = strings.TrimSpace(line[idx+1:])
			line = strings.TrimSpace(line[:idx])
		}

		tokens := strings.Fields(line)
		if len(tokens) < 4 {
			continue
		}

		imgIdx := parseInt(tokens[0])
		w := parseInt(tokens[1])
		h := parseInt(tokens[2])
		fc := parseInt(tokens[3])

		if fc < 1 || len(tokens) < 4+fc*2 {
			continue
		}

		fxs := make([]int, fc)
		fys := make([]int, fc)
		for j := 0; j < fc; j++ {
			fxs[j] = parseInt(tokens[4+j*2])
			fys[j] = parseInt(tokens[5+j*2])
		}

		tiles = append(tiles, Def{
			ImgIndex: imgIdx,
			Width:    w,
			Height:   h,
			FrameXs:  fxs,
			FrameYs:  fys,
			Comment:  comment,
		})
	}

	return &File{Images: images, Tiles: tiles}, nil
}
