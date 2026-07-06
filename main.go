// Command skein renders Claude Code's statusline: model name, context usage
// bar, 5h/weekly plan usage bars with reset countdowns, and a caveman badge.
// It replaces the original statusline.sh, and can install itself into
// settings.json.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/oxGrad/skein/internal/badge"
	"github.com/oxGrad/skein/internal/bar"
	statusctx "github.com/oxGrad/skein/internal/context"
	"github.com/oxGrad/skein/internal/install"
	"github.com/oxGrad/skein/internal/usage"
)

// fallbackContextWindow sizes the context bar when stdin doesn't carry
// context_window fields (old Claude Code, or early in a session).
const fallbackContextWindow = 200000

// staleAfter marks OAuth-cache plan data as stale when older than this.
const staleAfter = 10 * time.Minute

// version is injected by goreleaser via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			runInstall()
			return
		case "version", "--version", "-v":
			fmt.Println("skein " + versionString())
			return
		case "--refresh-usage-internal":
			refreshUsageCache(filepath.Join(claudeHome(), ".usage-cache.json"), filepath.Join(claudeHome(), ".credentials.json"))
			return
		}
	}
	runStatusline()
}

// versionString resolves the binary's version: the goreleaser-injected value
// when set, otherwise the module version stamped by `go install` (e.g.
// "v1.2.3"), otherwise "dev".
func versionString() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func claudeHome() string {
	if h := os.Getenv("CLAUDE_HOME"); h != "" {
		return h
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func runInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "skein install: resolve executable path:", err)
		os.Exit(1)
	}
	settingsPath := filepath.Join(claudeHome(), "settings.json")
	if err := install.Apply(settingsPath, exe); err != nil {
		fmt.Fprintln(os.Stderr, "skein install:", err)
		os.Exit(1)
	}
	fmt.Printf("statusLine command set to %s in %s\n", exe, settingsPath)
}

type rateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type stdinPayload struct {
	TranscriptPath string `json:"transcript_path"`
	Model          struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow struct {
		UsedPercentage *float64 `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *rateWindow `json:"five_hour"`
		SevenDay *rateWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

// planInfo is the resolved plan-usage data to display, whatever its source.
type planInfo struct {
	fiveHour  *int
	week      *int
	fiveReset string // countdown like "1h23m"; "" hides it
	weekReset string
	stale     bool // true when served from an OAuth cache older than staleAfter
}

// layout is one rung of the degradation ladder for narrow terminals.
type layout struct {
	ctxWidth    int
	planWidth   int
	show5h      bool
	showWk      bool
	show5hReset bool
	showWkReset bool
}

// layouts is tried in order until one fits the terminal width; the last is
// the floor. The 5h reset countdown is the most time-sensitive datum, so it
// survives every rung that still shows the 5h bar - the wk countdown, then
// the wk bar, then bar widths give way first.
var layouts = []layout{
	{ctxWidth: 10, planWidth: 10, show5h: true, showWk: true, show5hReset: true, showWkReset: true},
	{ctxWidth: 10, planWidth: 8, show5h: true, showWk: true, show5hReset: true},
	{ctxWidth: 10, planWidth: 8, show5h: true, showWk: false, show5hReset: true},
	{ctxWidth: 8, planWidth: 6, show5h: true, showWk: false, show5hReset: true},
	{ctxWidth: 6, planWidth: 0, show5h: false, showWk: false},
}

func runStatusline() {
	home := claudeHome()

	var payload stdinPayload
	if raw, err := io.ReadAll(os.Stdin); err == nil {
		json.Unmarshal(raw, &payload)
	}

	ctxPct := resolveCtxPct(payload)
	plan := resolvePlan(payload, home, time.Now())
	b := shortenBadge(badge.Render(home, badge.ExecRunner))

	cols, hasCols := terminalWidth()
	content := ""
	for i, l := range layouts {
		content = buildStatusline(payload.Model.DisplayName, ctxPct, plan, l)
		if !hasCols || i == len(layouts)-1 {
			break
		}
		need := visibleWidth(content) + rightAlignMargin()
		if b != "" {
			need += 1 + visibleWidth(b)
		}
		if need <= cols {
			break
		}
	}

	if b != "" {
		fmt.Println(rightAlignBadge(content, b))
		return
	}
	fmt.Println(content)
}

// resolveCtxPct prefers the pre-calculated percentage Claude Code sends on
// stdin (correct for any context window size); falls back to parsing the
// transcript against a 200k window for older versions or early-session
// nulls.
func resolveCtxPct(payload stdinPayload) int {
	if payload.ContextWindow.UsedPercentage != nil {
		return int(*payload.ContextWindow.UsedPercentage)
	}
	if payload.TranscriptPath == "" {
		return 0
	}
	f, err := os.Open(payload.TranscriptPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	return statusctx.Percent(statusctx.LastUsedTokens(f), fallbackContextWindow)
}

// resolvePlan prefers rate_limits from stdin (present for Pro/Max sessions
// after the first API response, always fresh, includes reset times); falls
// back to the OAuth usage cache, marking it stale when old.
func resolvePlan(payload stdinPayload, home string, now time.Time) *planInfo {
	rl := payload.RateLimits
	if rl.FiveHour != nil || rl.SevenDay != nil {
		p := &planInfo{}
		if rl.FiveHour != nil {
			v := int(rl.FiveHour.UsedPercentage)
			p.fiveHour = &v
			p.fiveReset = formatUntil(now.Unix(), rl.FiveHour.ResetsAt)
		}
		if rl.SevenDay != nil {
			v := int(rl.SevenDay.UsedPercentage)
			p.week = &v
			p.weekReset = formatUntil(now.Unix(), rl.SevenDay.ResetsAt)
		}
		return p
	}
	return loadCachedPlan(home, now)
}

// loadCachedPlan serves plan usage from the OAuth cache, refreshing it
// synchronously on first run or via a detached child when merely stale
// (goroutines die with the process, unlike bash's `&`).
func loadCachedPlan(home string, now time.Time) *planInfo {
	cachePath := filepath.Join(home, ".usage-cache.json")
	credsPath := filepath.Join(home, ".credentials.json")

	info, statErr := os.Stat(cachePath)
	exists := statErr == nil
	var age time.Duration
	if exists {
		age = now.Sub(info.ModTime())
	}

	if usage.NeedsRefresh(exists, age) {
		if !exists {
			refreshUsageCache(cachePath, credsPath)
		} else {
			spawnBackgroundRefresh()
		}
	}

	cached, ok := usage.LoadCache(cachePath)
	if !ok {
		return nil
	}
	return &planInfo{
		fiveHour: cached.FiveHour,
		week:     cached.Week,
		stale:    exists && age > staleAfter,
	}
}

// formatUntil renders the time from now until a unix-epoch reset as a short
// countdown ("3d4h", "1h23m", "45m", "<1m"), or "" if the reset has passed.
func formatUntil(now, resetsAt int64) string {
	d := resetsAt - now
	if d <= 0 {
		return ""
	}
	days := d / 86400
	hours := (d % 86400) / 3600
	mins := (d % 3600) / 60
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, mins)
	case mins > 0:
		return fmt.Sprintf("%dm", mins)
	default:
		return "<1m"
	}
}

// buildStatusline composes the statusline content (everything left of the
// badge) for one layout rung.
func buildStatusline(model string, ctxPct int, plan *planInfo, l layout) string {
	var out strings.Builder

	sep := ""
	if model != "" {
		fmt.Fprintf(&out, "\033[1;38;5;255m%s\033[0m", model)
		sep = " \033[38;5;250m│ "
	} else {
		sep = "\033[38;5;250m"
	}

	fmt.Fprintf(&out, "%sctx\033[0m %s", sep, bar.Render(ctxPct, l.ctxWidth))

	if plan == nil {
		return out.String()
	}

	mark := ""
	if plan.stale {
		mark = "?"
	}

	if l.show5h && plan.fiveHour != nil {
		fmt.Fprintf(&out, " \033[38;5;250m│ 5h%s\033[0m %s", mark, bar.Render(*plan.fiveHour, l.planWidth))
		if l.show5hReset && plan.fiveReset != "" {
			fmt.Fprintf(&out, " \033[38;5;245m%s\033[0m", plan.fiveReset)
		}
	}
	if l.showWk && plan.week != nil {
		fmt.Fprintf(&out, " \033[38;5;250m│ wk%s\033[0m %s", mark, bar.Render(*plan.week, l.planWidth))
		if l.showWkReset && plan.weekReset != "" {
			fmt.Fprintf(&out, " \033[38;5;245m%s\033[0m", plan.weekReset)
		}
	}

	return out.String()
}

// shortenBadge compresses the caveman hook's "[CAVEMAN...]" badge to
// "[CAVE...]", preserving any mode suffix and colors.
func shortenBadge(b string) string {
	return strings.Replace(b, "CAVEMAN", "CAVE", 1)
}

// spawnBackgroundRefresh launches a detached child process to refresh the
// usage cache.
func spawnBackgroundRefresh() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "--refresh-usage-internal")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Start()
}

// visibleWidth counts display columns in s, skipping ANSI SGR escape
// sequences (\033[...m).
func visibleWidth(s string) int {
	width := 0
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
		width++
	}
	return width
}

// terminalWidth reads Claude Code's COLUMNS env var (set on statusLine
// child processes since v2.1.153). Returns 0, false if unset/unparseable,
// since statuslines aren't attached to a real tty and can't query it
// directly.
func terminalWidth() (int, bool) {
	v := os.Getenv("COLUMNS")
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// rightAlignMargin reserves columns for Claude Code's own status-bar chrome
// (borders/padding it draws around our output), which COLUMNS doesn't
// account for. Overridable via SKEIN_MARGIN for other terminal setups.
func rightAlignMargin() int {
	if v := os.Getenv("SKEIN_MARGIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return 6
}

// rightAlignBadge appends badge to content, padded to sit near the right
// edge of the terminal (minus the margin for Claude Code's own chrome).
// Falls back to a single leading space when the terminal width is unknown
// or too narrow to fit both pieces.
func rightAlignBadge(content, badge string) string {
	cols, ok := terminalWidth()
	if !ok {
		return content + " " + badge
	}
	pad := cols - rightAlignMargin() - visibleWidth(content) - visibleWidth(badge)
	if pad < 1 {
		pad = 1
	}
	return content + strings.Repeat(" ", pad) + badge
}

func refreshUsageCache(cachePath, credsPath string) {
	token := usage.LoadToken(credsPath)
	if token == "" {
		return
	}
	client := &http.Client{Timeout: 4 * time.Second}
	plan, err := usage.Fetch(client, token)
	if err != nil {
		return
	}
	usage.SaveCache(cachePath, plan)
}
