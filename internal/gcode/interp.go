package gcode

import (
	"fmt"
	"math"
)

// Motion kinds.
const (
	MotionRapid = "rapid" // G0
	MotionFeed  = "feed"  // G1
)

// Point is a physical position in millimeters.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Segment is one straight move in physical millimeter space.
type Segment struct {
	Line   int     `json:"line"`
	Motion string  `json:"motion"`
	Start  Point   `json:"start"`
	End    Point   `json:"end"`
	Length float64 `json:"length"`
}

// Summary aggregates a whole program run.
type Summary struct {
	RapidLength float64 `json:"rapid_length"`
	FeedLength  float64 `json:"feed_length"`
	Final       Point   `json:"final"`
	BBoxMin     Point   `json:"bbox_min"`
	BBoxMax     Point   `json:"bbox_max"`
}

// Result is the full interpretation output.
type Result struct {
	Segments []Segment
	Summary  Summary
}

type machine struct {
	pos       Point    // physical mm
	offset    [3]float64 // G92 offset per axis: physical = coord*unit + offset
	motion    string
	absolute  bool
	unit      float64 // multiplier to mm
	segments  []Segment
	rapidLen  float64
	feedLen   float64
	bmin      Point
	bmax      Point
}

const inchMM = 25.4

// Interpret runs parsed lines and produces segments plus a summary.
func Interpret(lines []Line) (*Result, error) {
	m := &machine{
		motion:   MotionRapid,
		absolute: true,
		unit:     1.0,
	}
	for _, ln := range lines {
		if err := m.exec(ln); err != nil {
			return nil, err
		}
	}
	sum := Summary{
		RapidLength: m.rapidLen,
		FeedLength:  m.feedLen,
		Final:       m.pos,
		BBoxMin:     m.bmin,
		BBoxMax:     m.bmax,
	}
	return &Result{Segments: m.segments, Summary: sum}, nil
}

func (m *machine) exec(ln Line) error {
	var (
		axisVals  [3]float64
		axisSeen  [3]bool
		gMotion   = -1 // 0 or 1
		gDist     = -1 // 90 or 91
		gUnit     = -1 // 20 or 21
		hasG92    bool
	)
	for _, w := range ln.Words {
		switch w.Letter {
		case 'X', 'Y', 'Z':
			ax := int(w.Letter - 'X')
			if axisSeen[ax] {
				return &Error{Line: ln.Number, Msg: fmt.Sprintf("duplicate axis word %c", w.Letter)}
			}
			axisSeen[ax] = true
			axisVals[ax] = w.Value
		case 'G':
			g := int(math.Round(w.Value))
			if math.Abs(w.Value-float64(g)) > 1e-9 {
				return &Error{Line: ln.Number, Msg: fmt.Sprintf("unsupported G code %v", w.Value)}
			}
			switch g {
			case 0, 1:
				if gMotion >= 0 && gMotion != g {
					return &Error{Line: ln.Number, Msg: "conflicting motion modes G0/G1"}
				}
				gMotion = g
			case 90, 91:
				if gDist >= 0 && gDist != g {
					return &Error{Line: ln.Number, Msg: "conflicting distance modes G90/G91"}
				}
				gDist = g
			case 20, 21:
				if gUnit >= 0 && gUnit != g {
					return &Error{Line: ln.Number, Msg: "conflicting unit modes G20/G21"}
				}
				gUnit = g
			case 92:
				if hasG92 {
					return &Error{Line: ln.Number, Msg: "duplicate G92"}
				}
				hasG92 = true
			default:
				return &Error{Line: ln.Number, Msg: fmt.Sprintf("unsupported G code G%d", g)}
			}
		default:
			return &Error{Line: ln.Number, Msg: fmt.Sprintf("unknown word %c", w.Letter)}
		}
	}

	if hasG92 && gMotion >= 0 {
		return &Error{Line: ln.Number, Msg: "G92 not allowed on the same line as G0/G1"}
	}

	// Unit and distance mode changes take effect before axis values.
	if gUnit == 20 {
		m.unit = inchMM
	} else if gUnit == 21 {
		m.unit = 1.0
	}
	if gDist == 90 {
		m.absolute = true
	} else if gDist == 91 {
		m.absolute = false
	}
	if gMotion == 0 {
		m.motion = MotionRapid
	} else if gMotion == 1 {
		m.motion = MotionFeed
	}

	anyAxis := axisSeen[0] || axisSeen[1] || axisSeen[2]

	if hasG92 {
		if !anyAxis {
			return &Error{Line: ln.Number, Msg: "G92 requires at least one axis word"}
		}
		for ax := 0; ax < 3; ax++ {
			if axisSeen[ax] {
				m.offset[ax] = axisCoord(m.pos, ax) - axisVals[ax]*m.unit
			}
		}
		return nil
	}

	if !anyAxis {
		return nil // pure modal change, no path segment
	}

	start := m.pos
	for ax := 0; ax < 3; ax++ {
		if !axisSeen[ax] {
			continue
		}
		v := axisVals[ax] * m.unit
		if m.absolute {
			setAxis(&m.pos, ax, v+m.offset[ax])
		} else {
			setAxis(&m.pos, ax, axisCoord(m.pos, ax)+v)
		}
	}
	seg := Segment{
		Line:   ln.Number,
		Motion: m.motion,
		Start:  start,
		End:    m.pos,
		Length: dist(start, m.pos),
	}
	m.segments = append(m.segments, seg)
	if m.motion == MotionRapid {
		m.rapidLen += seg.Length
	} else {
		m.feedLen += seg.Length
	}
	m.growBBox(m.pos)
	return nil
}

func (m *machine) growBBox(p Point) {
	m.bmin.X = math.Min(m.bmin.X, p.X)
	m.bmin.Y = math.Min(m.bmin.Y, p.Y)
	m.bmin.Z = math.Min(m.bmin.Z, p.Z)
	m.bmax.X = math.Max(m.bmax.X, p.X)
	m.bmax.Y = math.Max(m.bmax.Y, p.Y)
	m.bmax.Z = math.Max(m.bmax.Z, p.Z)
}

func axisCoord(p Point, ax int) float64 {
	switch ax {
	case 0:
		return p.X
	case 1:
		return p.Y
	}
	return p.Z
}

func setAxis(p *Point, ax int, v float64) {
	switch ax {
	case 0:
		p.X = v
	case 1:
		p.Y = v
	default:
		p.Z = v
	}
}

func dist(a, b Point) float64 {
	dx, dy, dz := b.X-a.X, b.Y-a.Y, b.Z-a.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
