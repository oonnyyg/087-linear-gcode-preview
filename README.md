# pathview — linear G-code path preview

`pathview` interprets the linear-motion subset of a G-code program as a
physical toolpath in millimeters and writes a per-segment CSV plus a
summary JSON. It is a pure offline preview tool: it never estimates
machining time, never simulates dynamics, and never controls real
equipment. Standard library only.

## Usage

```sh
go run ./cmd/pathview <program.txt> <output-dir>
```

Writes `<output-dir>/segments.csv` and `<output-dir>/summary.json`.
On invalid input the tool exits non-zero and writes no output files.

```sh
go run ./cmd/pathview examples/demo.nc out
go test ./...
```

## Supported words

Only `G`, `X`, `Y`, `Z` words are accepted. A word is a letter followed
by a finite decimal number; case is ignored and spaces between words are
optional. Blank lines and non-nested `( ... )` comments are ignored.

- `G0` / `G1` — rapid / feed linear motion (modal)
- `G90` / `G91` — absolute / incremental coordinates (modal)
- `G20` / `G21` — inch / millimeter units (modal), 1 inch = 25.4 mm
- `G92` — set the coordinate value of the current physical point

Errors (reported with the source line number, exit code 1):

- unknown words or unsupported G codes
- unterminated comments
- duplicate axis words on one line
- conflicting commands from the same modal group on one line
  (`G0`/`G1`, `G90`/`G91`, `G20`/`G21`)
- `G92` without an axis word, or on the same line as `G0`/`G1`

## Coordinate semantics

- All reported geometry is physical millimeters. Unit (`G20`/`G21`) and
  distance (`G90`/`G91`) mode changes on a line take effect before that
  line's axis values.
- Initial state: physical position (0,0,0), zero `G92` offsets,
  millimeters, absolute mode, `G0` motion mode.
- Motion mode is modal: a line with axis words but no `G0`/`G1` moves
  with the current mode. Unmentioned axes keep their position. Lines
  with only modal changes produce no segment.
- `G92 Xv` assigns coordinate `v` (in current units) to the current
  physical point by storing a per-axis offset; it never moves. Axes not
  mentioned keep their offsets. Later absolute targets are shifted by
  the offset (`physical = value*unit + offset`); incremental moves are
  unaffected.

## Output

`segments.csv` columns: `line,motion,start_x,start_y,start_z,end_x,end_y,end_z,length`
— source line number, `rapid`/`feed`, physical start/end points, and
Euclidean length in mm. Zero-length moves are kept.

`summary.json`: accumulated `rapid_length` and `feed_length` (mm), the
`final` physical position, and `bbox_min`/`bbox_max`, the 3D bounding
box covering the initial point and every segment endpoint.
