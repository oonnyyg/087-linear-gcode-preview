// Package gcode parses and interprets a small subset of G-code for
// offline path preview. It never talks to real hardware.
package gcode

import (
	"fmt"
	"strings"
)

// Word is a single letter+number pair on a line.
type Word struct {
	Letter byte // uppercased
	Value  float64
}

// Line is a parsed program line.
type Line struct {
	Number int // 1-based source line number
	Words  []Word
}

// Error describes a parse or semantic problem with line context.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// ParseLines parses the whole program text into lines of words.
// It strips non-nested parenthesized comments and blank lines.
func ParseLines(text string) ([]Line, error) {
	var lines []Line
	raw := strings.Split(text, "\n")
	for i, src := range raw {
		num := i + 1
		words, err := parseLine(src)
		if err != nil {
			return nil, &Error{Line: num, Msg: err.Error()}
		}
		if len(words) > 0 {
			lines = append(lines, Line{Number: num, Words: words})
		}
	}
	return lines, nil
}

func parseLine(src string) ([]Word, error) {
	var words []Word
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '(':
			end := strings.IndexByte(src[i+1:], ')')
			if end < 0 {
				return nil, fmt.Errorf("unterminated comment")
			}
			i += end + 2
		case isLetter(c):
			letter := toUpper(c)
			i++
			start := i
			if i < n && (src[i] == '+' || src[i] == '-') {
				i++
			}
			digits := 0
			for i < n && src[i] >= '0' && src[i] <= '9' {
				i++
				digits++
			}
			if i < n && src[i] == '.' {
				i++
				for i < n && src[i] >= '0' && src[i] <= '9' {
					i++
					digits++
				}
			}
			if digits == 0 {
				return nil, fmt.Errorf("word %q has no number", string(letter))
			}
			var v float64
			if _, err := fmt.Sscanf(src[start:i], "%g", &v); err != nil {
				return nil, fmt.Errorf("bad number in word %q: %v", string(letter), err)
			}
			words = append(words, Word{Letter: letter, Value: v})
		default:
			return nil, fmt.Errorf("unexpected character %q", string(c))
		}
	}
	return words, nil
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func toUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}
