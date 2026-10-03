package gcode

import (
	"math"
	"strings"
	"testing"
)

func run(t *testing.T, src string) *Result {
	t.Helper()
	lines, err := ParseLines(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Interpret(lines)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	return res
}

func runErr(t *testing.T, src string) error {
	t.Helper()
	lines, err := ParseLines(src)
	if err != nil {
		return err
	}
	_, err = Interpret(lines)
	return err
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestBasicAbsoluteMoves(t *testing.T) {
	res := run(t, "G0 X10 Y0\nG1 X10 Y5\n")
	if len(res.Segments) != 2 {
		t.Fatalf("want 2 segments, got %d", len(res.Segments))
	}
	s0 := res.Segments[0]
	if s0.Line != 1 || s0.Motion != MotionRapid {
		t.Errorf("seg0 meta: %+v", s0)
	}
	if s0.End.X != 10 || s0.Length != 10 {
		t.Errorf("seg0: %+v", s0)
	}
	s1 := res.Segments[1]
	if s1.Motion != MotionFeed || s1.Length != 5 {
		t.Errorf("seg1: %+v", s1)
	}
	if res.Summary.RapidLength != 10 || res.Summary.FeedLength != 5 {
		t.Errorf("summary: %+v", res.Summary)
	}
	if res.Summary.Final != (Point{X: 10, Y: 5}) {
		t.Errorf("final: %+v", res.Summary.Final)
	}
	if res.Summary.BBoxMin != (Point{}) || res.Summary.BBoxMax != (Point{X: 10, Y: 5}) {
		t.Errorf("bbox: %+v", res.Summary)
	}
}

func TestModalMotionAndImplicitMove(t *testing.T) {
	// G1 persists; axis words alone still move; modal-only line makes no segment.
	res := run(t, "G1 X1\nX2\nG90\nX3\n")
	if len(res.Segments) != 3 {
		t.Fatalf("want 3 segments, got %d", len(res.Segments))
	}
	for _, s := range res.Segments {
		if s.Motion != MotionFeed {
			t.Errorf("motion: %+v", s)
		}
	}
	if res.Summary.Final.X != 3 {
		t.Errorf("final: %+v", res.Summary.Final)
	}
}

func TestIncrementalAndUnits(t *testing.T) {
	res := run(t, "G91 G20\nG0 X1\nG90 G21\nG1 X30\n")
	if len(res.Segments) != 2 {
		t.Fatalf("segments: %d", len(res.Segments))
	}
	if !near(res.Segments[0].End.X, 25.4) {
		t.Errorf("inch move: %+v", res.Segments[0].End)
	}
	// G90 G21 on the same line take effect before X30.
	if !near(res.Segments[1].End.X, 30) {
		t.Errorf("abs mm move: %+v", res.Segments[1].End)
	}
	if !near(res.Segments[1].Length, 30-25.4) {
		t.Errorf("length: %v", res.Segments[1].Length)
	}
}

func TestG92Offset(t *testing.T) {
	res := run(t, "G0 X10\nG92 X0\nG0 X5\nG91 X2\n")
	segs := res.Segments
	if len(segs) != 3 {
		t.Fatalf("segments: %d", len(segs))
	}
	// After G92 X0 at physical 10, absolute X5 -> physical 15.
	if !near(segs[1].End.X, 15) {
		t.Errorf("post-G92 abs: %+v", segs[1].End)
	}
	// Incremental move ignores the offset.
	if !near(segs[2].End.X, 17) {
		t.Errorf("post-G92 incr: %+v", segs[2].End)
	}
}

func TestZeroLengthMoveKept(t *testing.T) {
	res := run(t, "G0 X1\nX1\n")
	if len(res.Segments) != 2 {
		t.Fatalf("segments: %d", len(res.Segments))
	}
	if res.Segments[1].Length != 0 {
		t.Errorf("zero length: %+v", res.Segments[1])
	}
}

func TestCommentsAndNoSpaces(t *testing.T) {
	res := run(t, "(hello) g1x1y2 (trail)\n\n(only comment)\n")
	if len(res.Segments) != 1 {
		t.Fatalf("segments: %d", len(res.Segments))
	}
	if res.Segments[0].End.X != 1 || res.Segments[0].End.Y != 2 {
		t.Errorf("end: %+v", res.Segments[0].End)
	}
}

func TestErrors(t *testing.T) {
	cases := map[string]string{
		"unterminated comment": "G1 X1 (oops\n",
		"unknown word":         "F100 X1\n",
		"unknown G code":       "G2 X1\n",
		"duplicate axis":       "G1 X1 X2\n",
		"conflict motion":      "G0 G1 X1\n",
		"conflict distance":    "G90 G91\n",
		"conflict units":       "G20 G21\n",
		"G92 no axis":          "G92\n",
		"G92 with motion":      "G92 G1 X1\n",
		"bad number":           "G1 X\n",
	}
	for name, src := range cases {
		if err := runErr(t, src); err == nil {
			t.Errorf("%s: expected error", name)
		} else if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%s: error lacks line number: %v", name, err)
		}
	}
}

func TestErrorLineNumber(t *testing.T) {
	err := runErr(t, "G1 X1\nG1 X2\nQ3\n")
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("want line 3 error, got %v", err)
	}
}
