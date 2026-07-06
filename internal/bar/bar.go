// Package bar renders ANSI progress bars for the statusline.
package bar

import (
	"fmt"
	"strings"
)

// Color256 picks a 256-color code for a percentage (green/yellow/pink).
func Color256(pct int) int {
	switch {
	case pct < 60:
		return 148
	case pct < 85:
		return 220
	default:
		return 197
	}
}

// Clamp restricts pct to [0, 100].
func Clamp(pct int) int {
	if pct > 100 {
		return 100
	}
	if pct < 0 {
		return 0
	}
	return pct
}

// Filled computes how many of width cells should be filled for pct.
func Filled(pct, width int) int {
	return (pct*width + 50) / 100
}

// Bars renders the fill pattern only (no color/percent), e.g. "██░░".
func Bars(pct, width int) string {
	pct = Clamp(pct)
	filled := Filled(pct, width)

	var sb strings.Builder
	for i := range width {
		if i < filled {
			sb.WriteRune('█')
		} else {
			sb.WriteRune('░')
		}
	}
	return sb.String()
}

// Label formats the percentage text embedded in a bar, e.g. "72%".
func Label(pct int) string {
	return fmt.Sprintf("%d%%", Clamp(pct))
}

// LabelStart returns the starting index for centering label within width,
// or -1 if label doesn't fit.
func LabelStart(label string, width int) int {
	if len(label) > width {
		return -1
	}
	return (width - len(label)) / 2
}

// emptyBG is the background color used for label characters sitting over
// unfilled bar cells.
const emptyBG = 238

// LabelFG picks a foreground color for a label character, contrasting
// against its cell's background: dark text over a filled (bright) cell,
// light text over an empty (dark gray) cell.
func LabelFG(filledCell bool) int {
	if filledCell {
		return 232
	}
	return 250
}

// LabelBG picks a background color for a label character: the bar's own
// color when it sits over a filled cell, a neutral dark gray otherwise.
func LabelBG(filledCell bool, color int) int {
	if filledCell {
		return color
	}
	return emptyBG
}

// Render draws a colored bar of the given width for pct (0-100), with the
// percentage label centered inside the bar. Each label character's
// background dynamically tracks the fill state of the cell underneath it -
// filled cells shade the digit/glyph with the bar's own color, empty cells
// with a neutral gray. If the label doesn't fit the width, it's appended
// after the bar instead.
func Render(pct, width int) string {
	pct = Clamp(pct)
	color := Color256(pct)
	label := Label(pct)
	filled := Filled(pct, width)
	barRunes := []rune(Bars(pct, width))
	start := LabelStart(label, width)

	if start < 0 {
		return fmt.Sprintf("\033[38;5;%dm%s\033[0m \033[38;5;250m%s\033[0m", color, string(barRunes), label)
	}

	var sb strings.Builder
	for i, r := range barRunes {
		if i >= start && i < start+len(label) {
			filledCell := i < filled
			fg := LabelFG(filledCell)
			bg := LabelBG(filledCell, color)
			fmt.Fprintf(&sb, "\033[1m\033[38;5;%dm\033[48;5;%dm%c\033[0m", fg, bg, label[i-start])
		} else {
			fmt.Fprintf(&sb, "\033[38;5;%dm%c\033[0m", color, r)
		}
	}
	return sb.String()
}
