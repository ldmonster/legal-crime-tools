# legal crime tools

A CLI tool for extracting, rendering, and composing isometric tile sprites and map views from **Legal Crime** (Capone) game data files.

## Features

- **Tile extraction** — Slice sprite sheets into individual tile PNGs based on `.tile` descriptor files
- **Atlas generation** — Export source BMPs as PNGs and build unified block sprite sheets
- **Block type rendering** — Render any of the 38 block types as standalone isometric images
- **Full map rendering** — Compose complete isometric map views from `.map` files
- **XOR deobfuscation** — Transparently decodes XOR-obfuscated BMP files (`key 0xCD`) from original Capone/PICS assets
- **Dual BMP support** — Handles both 8-bit paletted and 24-bit true-color BMPs with automatic transparency keying
- **Key generation** — Generate, write, and verify LCData.bin license keys
- **EXE patching** — Replace hardcoded dead server IPs in LegalCrime.EXE with a custom IP

## Requirements

- Go 1.25+
- Legal Crime game data files (`.tile`, `.map`, BMP sprite sheets)

## Build

```sh
make build
```

The binary is output to `./bin/legal-crime-tools`.

## Usage

```
legal-crime-tools [command] [flags]
```

### Global Flags

| Flag | Default | Description |
|---|---|---|
| `--tile` | `Chicago.tile` | Path to `.tile` descriptor file |
| `--pics` | `Pics` | Directory containing BMP sprite sheets |
| `--out` | `tiles_out` | Output directory |
| `-v`, `--verbose` | `false` | Print progress messages |

### Commands

#### `extract` — Extract individual tile sprites

Parses the `.tile` descriptor and slices each tile frame from its source sprite sheet into a separate PNG. Output is organized by source image name.

```sh
legal-crime-tools extract --tile Chicago.tile --pics Pics --out tiles_out
```

Output structure:
```
tiles_out/
  ch1/
    Road.png
    Road_f01.png      # multiple frames get suffixed
    Sidewalk.png
    ...
  block1/
    ...
```

#### `atlas` — Build sprite atlases

Exports each source BMP as a PNG and stacks all `Block*.bmp` files into a single `blocks_unified.png`.

```sh
legal-crime-tools atlas --tile Chicago.tile --pics Pics --out tiles_out
```

Output:
```
tiles_out/_atlas/
  ch1.png
  block1.png
  ...
  blocks_unified.png   # all Block BMPs stacked vertically
```

#### `render-bt` — Render a single block type

Renders one of the 38 block types (0–37) as a standalone isometric image.

```sh
legal-crime-tools render-bt --bt 5 --tile Chicago.tile --pics Pics
```

| Flag | Default | Description |
|---|---|---|
| `--bt` | *(required)* | Block type index (0–37) |
| `--btout` | `<out>/bt_<N>.png` | Output PNG path |

#### `render-map` — Render a full map

Composes a complete isometric map view from a `.map` file. Empty cells default to tile 0 (Road).

```sh
legal-crime-tools render-map --map Maps/Chicago.map --tile Chicago.tile --pics Pics
```

| Flag | Default | Description |
|---|---|---|
| `--map` | *(required)* | Path to `.map` file |
| `--mapout` | `<out>/<name>_map.png` | Output PNG path |

#### `keygen` — Generate or verify license keys

Generates LCData.bin license keys, writes them to disk, or verifies existing key files.

```sh
legal-crime-tools keygen                        # generate 1 key
legal-crime-tools keygen -n 5                   # generate 5 keys
legal-crime-tools keygen --write LCData.bin     # write a single key to file
legal-crime-tools keygen --verify LCData.bin    # verify an existing key file
```

| Flag | Default | Description |
|---|---|---|
| `-n`, `--count` | `1` | Number of keys to generate |
| `--write` | | Write a single key to LCData.bin at this path |
| `--verify` | | Verify an existing LCData.bin file |

#### `patch` — Patch server IPs in EXE

Replaces hardcoded dead server IPs in LegalCrime.EXE with a custom IP address. Creates a `.orig` backup before patching.

```sh
legal-crime-tools patch LegalCrime.EXE 192.168.1.100   # patch all 4 IPs
legal-crime-tools patch LegalCrime.EXE --restore        # restore from backup
```

| Flag | Default | Description |
|---|---|---|
| `--restore` | `false` | Restore EXE from `.orig` backup |

## File Formats

### `.tile` Descriptor

Text file listing source BMP images and tile definitions:

```
<nImages>
ch1.bmp
Block1.bmp
...
<nTiles>
<imgIdx> <width> <height> <frameCount> <fx0> <fy0> [<fx1> <fy1> ...] ; <comment>
...
```

Each tile references a source image by index, defines sprite dimensions, and lists frame coordinates (X, Y pairs) within the source sheet.

### `.map` File

Text file defining a grid of block placements:

```
XDIM <width>
YDIM <height>
BLOCKS
<x> <y> <blockType>
...
END
```

### BMP Sprite Sheets

Two variants are supported:

| Source | Bit Depth | Obfuscated | Transparency |
|---|---|---|---|
| `Pics/` | 8-bit paletted | No | Palette index 254 |
| `Capone/PICS/` | 24-bit true-color | XOR `0xCD` | RGB `#4f374a` (R=79 G=55 B=74) |

Both variants produce identical transparency — the background color `#4f374a` at palette index 254 is keyed to alpha 0.

## Block Types

38 block types (BT 0–37) define the game's building/terrain vocabulary. Each has a grid size (Cols × Rows) and a base tile index.

| Range | Grid | Description |
|---|---|---|
| BT 0–7 | 12×12 | Interleaved city blocks (CH1 + CH variants) |
| BT 8–11 | 12×12 | Sequential city blocks (CH9–CH12) |
| BT 12–13 | 12×8 | Wide city blocks |
| BT 14–15 | 8×12 | Tall city blocks |
| BT 16–17 | 12×12 | Interleaved variant blocks |
| BT 18–19 | 4×4 | Park/CH composite |
| BT 20–23 | 8×8 | Medium composite blocks |
| BT 24–33 | 3×3 – 5×5 | Small CH blocks |
| BT 34 | 1×1 | Road/marker |
| BT 35–37 | 4×4 | Park blocks |

Tile index formula: `BaseTile + dc*Rows + (Rows - 1 - dr)` where `dc ∈ [0, Cols)` and `dr ∈ [0, Rows)`.

## Isometric Rendering

The renderer uses a diamond/isometric projection with:

- **Cell half-width**: 20 px
- **Cell half-height**: 10 px
- **Draw order**: Back-to-front diagonal traversal for correct layering
- **Auto-crop**: Output is trimmed to the drawn content area

Screen coordinates from grid position:
```
screenX = (col - row + mapHeight) * 20
screenY = (col + row) * 10 + topPad
```

## Project Structure

```
legal-crime-tools/
├── cmd/
│   └── legal-crime-tools/
│       └── main.go          # Entrypoint
├── internal/
│   ├── cmd/
│   │   ├── root.go          # CLI root command & global flags
│   │   ├── extract.go       # extract command
│   │   ├── atlas.go         # atlas command
│   │   ├── renderbt.go      # render-bt command
│   │   ├── rendermap.go     # render-map command
│   │   ├── keygen.go        # keygen command
│   │   └── patch.go         # patch command
│   ├── bmp/
│   │   └── reader.go        # BMP decoder (8-bit, 24-bit, XOR deobfuscation)
│   ├── tile/
│   │   └── tile.go          # .tile file parser & block type table
│   ├── mapfile/
│   │   └── mapfile.go       # .map file parser
│   ├── render/
│   │   └── renderer.go      # Isometric renderer & compositor
│   └── imageutil/
│       └── util.go          # Image utilities, BMP cache, PNG export
├── Makefile
├── go.mod
└── .gitignore
```
