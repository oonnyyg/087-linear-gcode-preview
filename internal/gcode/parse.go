package gcode

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

type Mode int

const (
	Rapid Mode = iota
	Linear
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Segment struct {
	Line     int     `json:"line"`
	Kind     string  `json:"kind"`
	Start    Point   `json:"start"`
	End      Point   `json:"end"`
	Distance float64 `json:"distance"`
}

type Summary struct {
	RapidLength   float64 `json:"rapid_length_mm"`
	LinearLength  float64 `json:"linear_length_mm"`
	FinalPosition Point   `json:"final_position_mm"`
	BoundingBox   Box     `json:"bounding_box_mm"`
}

type Box struct {
	Min Point `json:"min"`
	Max Point `json:"max"`
}

type Vector struct {
	X float64
	Y float64
	Z float64
}

type Result struct {
	Segments []Segment `json:"segments"`
	Summary  Summary   `json:"summary"`
}

type word struct {
	letter rune
	text   string
	value  float64
}

type state struct {
	position Point
	offset   Point
	absolute bool
	metric   bool
	motion   Mode
}

func ParseProgram(program string) (Result, error) {
	program = strings.TrimPrefix(program, "\uFEFF")
	lines := strings.Split(program, "\n")
	st := state{absolute: true, metric: true, motion: Rapid}
	result := Result{Segments: []Segment{}}
	box := Box{Min: st.position, Max: st.position}

	for index, line := range lines {
		lineNumber := index + 1
		line = strings.TrimSuffix(line, "\r")
		words, hasAxes, err := scanLine(line, lineNumber)
		if err != nil {
			return Result{}, err
		}
		if err := applyLine(&st, words, hasAxes, &result, &box, lineNumber); err != nil {
			return Result{}, err
		}
	}

	result.Summary.BoundingBox = box
	result.Summary.FinalPosition = st.position
	return result, nil
}

func scanLine(line string, lineNumber int) ([]word, map[rune]bool, error) {
	words := []word{}
	hasAxes := map[rune]bool{}
	runes := []rune(line)
	for i := 0; i < len(runes); {
		switch {
		case unicode.IsSpace(runes[i]):
			i++
		case runes[i] == '(':
			closed := false
			i++
			for i < len(runes) {
				if runes[i] == '(' {
					return nil, nil, lineError(lineNumber, i+1, "nested comment")
				}
				if runes[i] == ')' {
					closed = true
					i++
					break
				}
				i++
			}
			if !closed {
				return nil, nil, fmt.Errorf("line %d: unclosed comment", lineNumber)
			}
		case isLetter(runes[i]):
			letter := unicode.ToUpper(runes[i])
			start := i
			i++
			numStart := i
			if i < len(runes) && (runes[i] == '+' || runes[i] == '-') {
				i++
			}
			digitsBefore := 0
			for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
				digitsBefore++
				i++
			}
			digitsAfter := 0
			if i < len(runes) && runes[i] == '.' {
				i++
				for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
					digitsAfter++
					i++
				}
			}
			if digitsBefore == 0 && digitsAfter == 0 {
				return nil, nil, lineError(lineNumber, start+1, "word must contain a finite decimal number")
			}
			text := string(runes[numStart:i])
			value, err := strconv.ParseFloat(text, 64)
			if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
				return nil, nil, lineError(lineNumber, start+1, "word must contain a finite decimal number")
			}
			if letter == 'X' || letter == 'Y' || letter == 'Z' {
				if hasAxes[letter] {
					return nil, nil, lineError(lineNumber, start+1, "duplicate axis word %c", letter)
				}
				hasAxes[letter] = true
			}
			words = append(words, word{letter: letter, text: text, value: value})
		default:
			return nil, nil, lineError(lineNumber, i+1, "unexpected character %q", runes[i])
		}
	}
	return words, hasAxes, nil
}

func applyLine(st *state, words []word, hasAxes map[rune]bool, result *Result, box *Box, lineNumber int) error {
	unitSet, distanceSet, motionSet := false, false, false
	var unitMetric, distanceAbsolute bool
	var motion Mode
	hasG92 := false

	for _, w := range words {
		if w.letter != 'G' {
			continue
		}
		switch w.value {
		case 0:
			if motionSet && motion != Rapid {
				return lineError(lineNumber, 0, "conflicting motion mode")
			}
			motion, motionSet = Rapid, true
		case 1:
			if motionSet && motion != Linear {
				return lineError(lineNumber, 0, "conflicting motion mode")
			}
			motion, motionSet = Linear, true
		case 20:
			if unitSet && unitMetric {
				return lineError(lineNumber, 0, "conflicting unit mode")
			}
			unitMetric, unitSet = false, true
		case 21:
			if unitSet && !unitMetric {
				return lineError(lineNumber, 0, "conflicting unit mode")
			}
			unitMetric, unitSet = true, true
		case 90:
			if distanceSet && !distanceAbsolute {
				return lineError(lineNumber, 0, "conflicting distance mode")
			}
			distanceAbsolute, distanceSet = true, true
		case 91:
			if distanceSet && distanceAbsolute {
				return lineError(lineNumber, 0, "conflicting distance mode")
			}
			distanceAbsolute, distanceSet = false, true
		case 92:
			hasG92 = true
		default:
			return lineError(lineNumber, 0, "unknown word G%s", w.text)
		}
	}
	for _, w := range words {
		if w.letter == 'G' {
			continue
		}
		if w.letter != 'X' && w.letter != 'Y' && w.letter != 'Z' {
			return lineError(lineNumber, 0, "unknown word %c%s", w.letter, w.text)
		}
	}
	if hasG92 && motionSet {
		return lineError(lineNumber, 0, "G92 cannot share a line with a motion command")
	}
	if hasG92 && len(hasAxes) == 0 {
		return lineError(lineNumber, 0, "G92 requires at least one axis")
	}

	if unitSet {
		st.metric = unitMetric
	}
	if distanceSet {
		st.absolute = distanceAbsolute
	}
	if motionSet {
		st.motion = motion
	}
	scale := 1.0
	if !st.metric {
		scale = 25.4
	}

	if hasG92 {
		for _, w := range words {
			if w.letter != 'X' && w.letter != 'Y' && w.letter != 'Z' {
				continue
			}
			logical := w.value * scale
			offset := st.position.axis(w.letter) - logical
			st.offset = setAxis(st.offset, w.letter, offset)
		}
		return nil
	}

	if len(hasAxes) == 0 {
		return nil
	}
	start := st.position
	end := applyAxes(start, st.offset, words, st.absolute, scale)
	distance := length(Vector{X: end.X - start.X, Y: end.Y - start.Y, Z: end.Z - start.Z})
	kind := "G1"
	if st.motion == Rapid {
		kind = "G0"
		result.Summary.RapidLength += distance
	} else {
		result.Summary.LinearLength += distance
	}
	result.Segments = append(result.Segments, Segment{
		Line:     lineNumber,
		Kind:     kind,
		Start:    start,
		End:      end,
		Distance: distance,
	})
	st.position = end
	expandBox(box, end)
	return nil
}

func applyAxes(position Point, offset Point, words []word, absolute bool, scale float64) Point {
	result := position
	for _, w := range words {
		if w.letter != 'X' && w.letter != 'Y' && w.letter != 'Z' {
			continue
		}
		physical := w.value * scale
		if absolute {
			physical += offset.axis(w.letter)
		} else {
			physical += position.axis(w.letter)
		}
		result = setAxis(result, w.letter, physical)
	}
	return result
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func (p Point) axis(axis rune) float64 {
	switch axis {
	case 'X':
		return p.X
	case 'Y':
		return p.Y
	default:
		return p.Z
	}
}

func setAxis(p Point, axis rune, value float64) Point {
	switch axis {
	case 'X':
		p.X = value
	case 'Y':
		p.Y = value
	default:
		p.Z = value
	}
	return p
}

func length(v Vector) float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

func expandBox(box *Box, p Point) {
	box.Min.X = math.Min(box.Min.X, p.X)
	box.Min.Y = math.Min(box.Min.Y, p.Y)
	box.Min.Z = math.Min(box.Min.Z, p.Z)
	box.Max.X = math.Max(box.Max.X, p.X)
	box.Max.Y = math.Max(box.Max.Y, p.Y)
	box.Max.Z = math.Max(box.Max.Z, p.Z)
}

func lineError(line int, column int, format string, args ...any) error {
	if column > 0 {
		return fmt.Errorf("line %d column %d: %s", line, column, fmt.Sprintf(format, args...))
	}
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}
