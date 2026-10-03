// Command pathview interprets linear G-code motion as a physical
// millimeter path and writes per-segment CSV plus a summary JSON.
// It is an offline preview tool and never controls real hardware.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"example.com/linear-gcode-preview/internal/gcode"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: pathview <program.txt> <output-dir>\n")
		os.Exit(2)
	}
	input, outDir := os.Args[1], os.Args[2]

	data, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input: %v\n", err)
		os.Exit(1)
	}
	lines, err := gcode.ParseLines(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}
	res, err := gcode.Interpret(lines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "interpret error: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir: %v\n", err)
		os.Exit(1)
	}
	if err := writeCSV(filepath.Join(outDir, "segments.csv"), res.Segments); err != nil {
		fmt.Fprintf(os.Stderr, "write csv: %v\n", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(outDir, "summary.json"), res.Summary); err != nil {
		fmt.Fprintf(os.Stderr, "write json: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ok: %d segments, rapid %.3f mm, feed %.3f mm\n",
		len(res.Segments), res.Summary.RapidLength, res.Summary.FeedLength)
}

func writeCSV(path string, segs []gcode.Segment) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{
		"line", "motion",
		"start_x", "start_y", "start_z",
		"end_x", "end_y", "end_z",
		"length",
	}); err != nil {
		return err
	}
	for _, s := range segs {
		rec := []string{
			strconv.Itoa(s.Line), s.Motion,
			num(s.Start.X), num(s.Start.Y), num(s.Start.Z),
			num(s.End.X), num(s.End.Y), num(s.End.Z),
			num(s.Length),
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return w.Error()
}

func num(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func writeJSON(path string, sum gcode.Summary) error {
	data, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
