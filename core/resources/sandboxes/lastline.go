package sandboxes

import (
	"regexp"
	"strings"
	"unicode"
)

// maxLineRunes caps LastLine.
const maxLineRunes = 120

// escapes matches terminal control sequences: CSI (colors, cursor moves),
// OSC (titles, ended by BEL or ST) and two-byte escapes.
var escapes = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// LastLine is the last non-blank line of raw terminal output, with control
// sequences removed, a carriage return keeping only what follows it (a
// redrawn line), and at most maxLineRunes runes kept. A line with only a
// prompt such as "> " is kept as written, trailing space trimmed.
func LastLine(data []byte) string {
	text := escapes.ReplaceAllString(string(data), "")
	lines := strings.Split(text, "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		// A redraw: what follows the last non-final carriage return.
		line = strings.TrimRight(line, "\r")
		cut := strings.LastIndex(line, "\r")
		if cut >= 0 {
			line = line[cut+1:]
		}
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && r != '\t' {
				return -1
			}
			return r
		}, line)
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.TrimSpace(line) == "" {
			continue
		}

		runes := []rune(line)
		if len(runes) > maxLineRunes {
			line = string(runes[:maxLineRunes]) + "…"
		}
		return line
	}

	return ""
}
