package bar

import (
	"fmt"
	"strings"
	"testing"
)

func TestClamp(t *testing.T) {
	cases := map[int]int{-5: 0, 0: 0, 50: 50, 100: 100, 150: 100}
	for in, want := range cases {
		if got := Clamp(in); got != want {
			t.Errorf("Clamp(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestColor256(t *testing.T) {
	cases := []struct {
		pct  int
		want int
	}{
		{0, 148}, {59, 148}, {60, 220}, {84, 220}, {85, 197}, {100, 197},
	}
	for _, c := range cases {
		if got := Color256(c.pct); got != c.want {
			t.Errorf("Color256(%d) = %d, want %d", c.pct, got, c.want)
		}
	}
}

func TestFilled(t *testing.T) {
	cases := []struct{ pct, width, want int }{
		{0, 10, 0},
		{100, 10, 10},
		{50, 10, 5},
		{33, 10, 3},
	}
	for _, c := range cases {
		if got := Filled(c.pct, c.width); got != c.want {
			t.Errorf("Filled(%d, %d) = %d, want %d", c.pct, c.width, got, c.want)
		}
	}
}

func TestBars(t *testing.T) {
	got := Bars(50, 10)
	if strings.Count(got, "█") != 5 || strings.Count(got, "░") != 5 {
		t.Errorf("Bars(50, 10) = %q, want 5 filled + 5 empty", got)
	}
	if got := Bars(150, 4); strings.Count(got, "█") != 4 {
		t.Errorf("Bars clamps pct>100, got %q", got)
	}
}

func TestRenderContainsPercentAndColor(t *testing.T) {
	got := Render(72, 5)
	if !strings.Contains(stripANSI(got), "72%") {
		t.Errorf("Render(72, 5) = %q, want visible text to contain 72%%", got)
	}
	if !strings.Contains(got, "\033[38;5;220m") {
		t.Errorf("Render(72, 5) = %q, want yellow color code", got)
	}
}

func TestLabel(t *testing.T) {
	cases := map[int]string{0: "0%", 50: "50%", 100: "100%", 150: "100%", -10: "0%"}
	for in, want := range cases {
		if got := Label(in); got != want {
			t.Errorf("Label(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestLabelStart(t *testing.T) {
	cases := []struct {
		label string
		width int
		want  int
	}{
		{"50%", 10, 3},
		{"100%", 5, 0},
		{"100%", 4, 0},
		{"100%", 3, -1},
	}
	for _, c := range cases {
		if got := LabelStart(c.label, c.width); got != c.want {
			t.Errorf("LabelStart(%q, %d) = %d, want %d", c.label, c.width, got, c.want)
		}
	}
}

func TestLabelFG(t *testing.T) {
	if got := LabelFG(true); got != 232 {
		t.Errorf("LabelFG(true) = %d, want 232", got)
	}
	if got := LabelFG(false); got != 250 {
		t.Errorf("LabelFG(false) = %d, want 250", got)
	}
}

func TestLabelBG(t *testing.T) {
	if got := LabelBG(true, 148); got != 148 {
		t.Errorf("LabelBG(true, 148) = %d, want 148", got)
	}
	if got := LabelBG(false, 148); got != emptyBG {
		t.Errorf("LabelBG(false, 148) = %d, want %d", got, emptyBG)
	}
}

func TestRenderLabelBackgroundTracksFillState(t *testing.T) {
	// pct=30, width=10 -> filled=3, label "30%" (3 chars) centered at index 3,
	// spanning indices 3,4,5 - all within the filled region (0..2 filled...
	// wait filled=3 means indices 0,1,2 filled). Use a pct/width combo where
	// the label straddles the fill boundary to exercise both branches.
	got := Render(30, 10) // filled=3, label "30%" starts at (10-3)/2=3, spans 3,4,5 (all empty)
	if !strings.Contains(got, fmt.Sprintf("\033[48;5;%dm", emptyBG)) {
		t.Errorf("Render(30, 10) = %q, want at least one empty-cell background code", got)
	}

	got2 := Render(90, 10) // filled=9, label "90%" starts at (10-3)/2=3, spans 3,4,5 (all filled)
	color := Color256(90)
	if !strings.Contains(got2, fmt.Sprintf("\033[48;5;%dm", color)) {
		t.Errorf("Render(90, 10) = %q, want at least one filled-cell background code %d", got2, color)
	}
}

func TestRenderLabelFitsInsideBar(t *testing.T) {
	got := Render(50, 10)
	// Strip ANSI codes to check the visible label sits within bar-width chars.
	visible := stripANSI(got)
	if strings.Contains(got, " 50%") {
		t.Errorf("Render(50, 10) = %q, label should be embedded, not trailing", got)
	}
	if !strings.Contains(visible, "50%") {
		t.Errorf("visible output %q missing label", visible)
	}
	if len([]rune(visible)) != 10 {
		t.Errorf("visible output %q length = %d, want 10 (bar width, no trailing label)", visible, len([]rune(visible)))
	}
}

func TestRenderFallsBackWhenLabelTooWide(t *testing.T) {
	got := Render(100, 3)
	if !strings.Contains(got, "100%") {
		t.Errorf("Render(100, 3) = %q, want fallback trailing 100%%", got)
	}
}

func stripANSI(s string) string {
	var sb strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
