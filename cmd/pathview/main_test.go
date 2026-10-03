package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"example.com/linear-gcode-preview/internal/gcode"
)

func TestRunWritesCSVAndSummary(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "program.gcode")
	output := filepath.Join(root, "out")
	if err := os.WriteFile(input, []byte("G90 G21 G0 X3 Y4\nG1 Z0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"-input", input, "-output", output}); err != nil {
		t.Fatal(err)
	}

	csvFile, err := os.Open(filepath.Join(output, "segments.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer csvFile.Close()
	records, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d CSV rows", len(records))
	}
	if records[1][1] != "G0" || records[2][1] != "G1" {
		t.Fatalf("unexpected motion rows: %#v", records[1:])
	}

	summaryData, err := os.ReadFile(filepath.Join(output, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary gcode.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.RapidLength != 5 || summary.LinearLength != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestRunInvalidInputCreatesNoOutput(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "program.gcode")
	output := filepath.Join(root, "missing-output")
	if err := os.WriteFile(input, []byte("G1 (unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"-input", input, "-output", output}); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output directory exists or stat failed: %v", err)
	}
}
