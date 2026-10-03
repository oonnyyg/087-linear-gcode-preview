package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"example.com/linear-gcode-preview/internal/gcode"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pathview:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("pathview", flag.ContinueOnError)
	inputPath := flags.String("input", "", "path to the G-code text file")
	outputDir := flags.String("output", "", "directory for segments.csv and summary.json")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" || *outputDir == "" {
		return errors.New("usage: pathview -input program.gcode -output directory")
	}

	file, err := os.Open(*inputPath)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	result, err := gcode.ParseProgram(string(data))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		return err
	}
	if err := writeSegments(filepath.Join(*outputDir, "segments.csv"), result); err != nil {
		return err
	}
	if err := writeSummary(filepath.Join(*outputDir, "summary.json"), result.Summary); err != nil {
		return err
	}
	return nil
}

func writeSegments(path string, result gcode.Result) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	header := []string{
		"line", "motion",
		"start_x_mm", "start_y_mm", "start_z_mm",
		"end_x_mm", "end_y_mm", "end_z_mm",
		"length_mm",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, segment := range result.Segments {
		record := []string{
			strconv.Itoa(segment.Line),
			segment.Kind,
			number(segment.Start.X), number(segment.Start.Y), number(segment.Start.Z),
			number(segment.End.X), number(segment.End.Y), number(segment.End.Z),
			number(segment.Distance),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeSummary(path string, summary gcode.Summary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func number(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}
