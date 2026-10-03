package gcode

import (
	"math"
	"strings"
	"testing"
)

func TestParseBasicMotionsAndBoundingBox(t *testing.T) {
	result, err := ParseProgram("G0 X3 Y4\nG1 Z0\nX0 Y0 Z0\n")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Segments) != 3 {
		t.Fatalf("got %d segments", len(result.Segments))
	}
	assertPoint(t, result.Segments[0].End, Point{X: 3, Y: 4, Z: 0})
	assertFloat(t, result.Segments[0].Distance, 5)
	if result.Segments[0].Kind != "G0" {
		t.Fatalf("kind = %q", result.Segments[0].Kind)
	}
	assertFloat(t, result.Summary.RapidLength, 5)
	assertFloat(t, result.Summary.LinearLength, 5)
	assertPoint(t, result.Summary.FinalPosition, Point{})
	assertPoint(t, result.Summary.BoundingBox.Min, Point{})
	assertPoint(t, result.Summary.BoundingBox.Max, Point{X: 3, Y: 4, Z: 0})
}

func TestParseUnitsDistanceModesAndG92(t *testing.T) {
	program := strings.Join([]string{
		"G20 G91 G0 X1",
		"G21 G90 G92 X10",
		"G90 X10 Y2",
		"G91 X1",
		"X0",
	}, "\n")
	result, err := ParseProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	assertPoint(t, result.Segments[0].End, Point{X: 25.4})
	assertPoint(t, result.Segments[1].End, Point{X: 25.4, Y: 2})
	assertPoint(t, result.Segments[2].End, Point{X: 26.4, Y: 2})
	assertPoint(t, result.Segments[3].End, Point{X: 26.4, Y: 2})
	assertFloat(t, result.Segments[3].Distance, 0)
}

func TestParseCommentsNoSpacesAndRepeatedModes(t *testing.T) {
	program := "(first) g20g91x1 Y2 (tail)\nG20 G91 G00 g00 x0\n"
	result, err := ParseProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) != 2 {
		t.Fatalf("got %d segments", len(result.Segments))
	}
	assertPoint(t, result.Segments[0].End, Point{X: 25.4, Y: 50.8})
	assertFloat(t, result.Segments[1].Distance, 0)
}

func TestParseRejectsInvalidPrograms(t *testing.T) {
	tests := []struct {
		name    string
		program string
		want    string
	}{
		{"unknown word", "Q1\n", "line 1"},
		{"unsupported g code", "G2 X1\n", "line 1"},
		{"unclosed comment", "G1 (note\n", "line 1"},
		{"nested comment", "G1 (a (b))\n", "line 1"},
		{"duplicate axis", "G1 X1 X2\n", "line 1"},
		{"motion conflict", "G0 G1 X1\n", "line 1"},
		{"unit conflict", "G20 G21\n", "line 1"},
		{"distance conflict", "G90 G91\n", "line 1"},
		{"g92 without axis", "G92\n", "line 1"},
		{"g92 with motion", "G1 G92 X1\n", "line 1"},
		{"error reports later line", "\n\nG90 G91\n", "line 3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseProgram(test.program)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want prefix containing %q", err, test.want)
			}
		})
	}
}

func assertPoint(t *testing.T, got, want Point) {
	t.Helper()
	assertFloat(t, got.X, want.X)
	assertFloat(t, got.Y, want.Y)
	assertFloat(t, got.Z, want.Z)
}

func assertFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
