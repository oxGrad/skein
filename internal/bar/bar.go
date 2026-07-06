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

// LabelFG picks a foreground color for a label character, contrasting
// against its cell's background: dark text over a filled (bright) cell,
// near-white text over an empty cell's dim background.
func LabelFG(filledCell bool, color int) int {
	if filledCell {
		return 232
	}
	return 255
}

// UnfilledBG picks the background for label characters over unfilled cells:
// the dim 256-color shade that the ░ glyph (25% ink over a dark terminal)
// visually approximates for each bar color.
func UnfilledBG(color int) int {
	if color == 197 {
		return 52
	}
	return 58
}

// Render draws a colored bar of the given width for pct (0-100), with the
// percentage label centered inside the bar. Over a filled cell the label
// takes the bar's color as its background (dark text on bright color);
// over an empty cell the background is a dim shade approximating the ░
// glyph, with the bar's own color as foreground. Runs of identically-styled
// cells share one escape sequence to keep output small - the statusline
// re-renders on every keystroke. If the label doesn't fit the width, it's
// appended after the bar instead.
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
			bg := color
			if !filledCell {
				bg = UnfilledBG(color)
			}
			style := fmt.Sprintf("\033[1;38;5;%d;48;5;%dm", LabelFG(filledCell, color), bg)
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
