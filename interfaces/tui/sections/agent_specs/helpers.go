package agent_specs

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/mattn/go-runewidth"
)

const (
	agentSpecsPageSize = 20
)

type columnSpec struct {
	title string
	min   int
	flex  int
}

var agentSpecColumnSpecs = []columnSpec{
	{title: "Name", min: 24, flex: 3},
	{title: "Provider", min: 12, flex: 1},
	{title: "Model", min: 18, flex: 2},
	{title: "Reasoning", min: 12, flex: 1},
	{title: "Version", min: 8, flex: 1},
	{title: "Spec ID", min: 36, flex: 3},
}

func buildAgentSpecColumns(totalWidth int) []table.Column {
	minSum := 0
	totalFlex := 0
	for _, spec := range agentSpecColumnSpecs {
		minSum += spec.min
		totalFlex += spec.flex
	}

	extra := totalWidth - minSum
	if extra < 0 {
		extra = 0
	}
	if totalFlex == 0 {
		totalFlex = 1
	}

	cols := make([]table.Column, len(agentSpecColumnSpecs))
	for i, spec := range agentSpecColumnSpecs {
		width := spec.min
		if extra > 0 {
			width += extra * spec.flex / totalFlex
		}
		if width < spec.min {
			width = spec.min
		}
		cols[i] = table.Column{Title: spec.title, Width: width}
	}
	return cols
}

func wrapText(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return value
	}

	lines := strings.Split(value, "\n")
	var wrapped []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			wrapped = append(wrapped, "")
			continue
		}

		words := strings.Fields(line)
		var current strings.Builder
		currentWidth := 0

		flushCurrent := func() {
			if current.Len() == 0 {
				return
			}
			wrapped = append(wrapped, current.String())
			current.Reset()
			currentWidth = 0
		}

		for _, word := range words {
			wordWidth := runewidth.StringWidth(word)
			if currentWidth == 0 {
				if wordWidth <= maxWidth {
					current.WriteString(word)
					currentWidth = wordWidth
					continue
				}
				wrapped = append(wrapped, wrapLongWord(word, maxWidth)...)
				continue
			}

			if currentWidth+1+wordWidth <= maxWidth {
				current.WriteByte(' ')
				current.WriteString(word)
				currentWidth += 1 + wordWidth
				continue
			}

			flushCurrent()
			if wordWidth <= maxWidth {
				current.WriteString(word)
				currentWidth = wordWidth
			} else {
				wrapped = append(wrapped, wrapLongWord(word, maxWidth)...)
			}
		}

		flushCurrent()
	}

	return strings.Join(wrapped, "\n")
}

func wrapLongWord(word string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{word}
	}

	var (
		segments     []string
		current      strings.Builder
		currentWidth int
	)

	flush := func() {
		if current.Len() == 0 {
			return
		}
		segments = append(segments, current.String())
		current.Reset()
		currentWidth = 0
	}

	for _, r := range word {
		rw := runewidth.RuneWidth(r)
		if rw == 0 {
			rw = 1
		}

		if currentWidth+rw > maxWidth && current.Len() > 0 {
			flush()
		}

		current.WriteRune(r)
		currentWidth += rw
	}

	flush()
	return segments
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
