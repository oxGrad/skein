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
// with a neutral gray. On a fully empty bar the label drops its background
// entirely (gray-on-gray reads poorly). Runs of identically-styled cells
// share one escape sequence to keep output small - the statusline re-renders
// on every keystroke. If the label doesn't fit the width, it's appended
// after the bar instead.
func Render(pct, width int) string {
	pct = Clamp(pct)
	color := Color256(pct)
	label := Label(pct)
	filled := Filled(pct, width)
	start := LabelStart(label, width)

	if start < 0 {
		return fmt.Sprintf("\033[38;5;%dm%s\033[0m \033[38;5;250m%s\033[0m", color, Bars(pct, width), label)
	}

	barStyle := fmt.Sprintf("\033[38;5;%dm", color)
	var sb strings.Builder
	cur := ""
	emit := func(style string, ch byte, r rune) {
		if style != cur {
			if cur != "" {
				sb.WriteString("\033[0m")
			}
			sb.WriteString(style)
			cur = style
		}
		if ch != 0 {
			sb.WriteByte(ch)
		} else {
			sb.WriteRune(r)
		}
	}

	for i := range width {
		if i >= start && i < start+len(label) {
			filledCell := i < filled
			var style string
			if filled == 0 {
				style = "\033[1;38;5;250m"
			} else {
				style = fmt.Sprintf("\033[1;38;5;%d;48;5;%dm", LabelFG(filledCell), LabelBG(filledCell, color))
			}
			emit(style, label[i-start], 0)
		} else if i < filled {
			emit(barStyle, 0, '█')
		} else {
			emit(barStyle, 0, '░')
		}
	}
	sb.WriteString("\033[0m")
	return sb.String()
}
