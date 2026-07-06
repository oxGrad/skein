// Command skein renders Claude Code's statusline: a caveman badge, context
// usage bar, and 5h/weekly plan usage bars. It replaces the original
// statusline.sh, and can install itself into settings.json.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/oxGrad/skein/internal/badge"
	"github.com/oxGrad/skein/internal/bar"
	statusctx "github.com/oxGrad/skein/internal/context"
	"github.com/oxGrad/skein/internal/install"
	"github.com/oxGrad/skein/internal/usage"
)

const contextWindow = 200000

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			runInstall()
			return
		case "--refresh-usage-internal":
			refreshUsageCache(filepath.Join(claudeHome(), ".usage-cache.json"), filepath.Join(claudeHome(), ".credentials.json"))
			return
		}
	}
	runStatusline()
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

type stdinPayload struct {
	TranscriptPath string `json:"transcript_path"`
	Model          struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
}

func runStatusline() {
	home := claudeHome()

	var payload stdinPayload
	if raw, err := io.ReadAll(os.Stdin); err == nil {
		json.Unmarshal(raw, &payload)
	}

	var out strings.Builder

	hasModel := payload.Model.DisplayName != ""
	if hasModel {
		fmt.Fprintf(&out, "\033[38;5;250m%s\033[0m", payload.Model.DisplayName)
	}

	ctxPct := 0
	if payload.TranscriptPath != "" {
		if f, err := os.Open(payload.TranscriptPath); err == nil {
			used := statusctx.LastUsedTokens(f)
			f.Close()
			ctxPct = statusctx.Percent(used, contextWindow)
		}
	}
	if hasModel {
		fmt.Fprintf(&out, " \033[38;5;250m│ ctx\033[0m %s", bar.Render(ctxPct, 10))
	} else {
		fmt.Fprintf(&out, "\033[38;5;250mctx\033[0m %s", bar.Render(ctxPct, 10))
	}

	if plan, ok := loadOrRefreshUsage(home); ok {
		if plan.FiveHour != nil {
			fmt.Fprintf(&out, " \033[38;5;250m│ 5h\033[0m %s", bar.Render(*plan.FiveHour, 8))
		}
		if plan.Week != nil {
			fmt.Fprintf(&out, " \033[38;5;250m│ wk\033[0m %s", bar.Render(*plan.Week, 8))
		}
	}

	content := out.String()
	if b := badge.Render(home, badge.ExecRunner); b != "" {
		fmt.Println(rightAlignBadge(content, b))
		return
	}

	fmt.Println(content)
}

// loadOrRefreshUsage returns cached plan usage, refreshing the cache
// synchronously if it's missing (first run) or asynchronously in the
// background if merely stale, matching the shell script's non-blocking
// refresh behavior.
func loadOrRefreshUsage(home string) (usage.Plan, bool) {
	cachePath := filepath.Join(home, ".usage-cache.json")
	credsPath := filepath.Join(home, ".credentials.json")

	info, statErr := os.Stat(cachePath)
	exists := statErr == nil
	var age time.Duration
	if exists {
		age = time.Since(info.ModTime())
	}

	if !usage.NeedsRefresh(exists, age) {
		return usage.LoadCache(cachePath)
	}

	if !exists {
		refreshUsageCache(cachePath, credsPath)
		return usage.LoadCache(cachePath)
	}

	spawnBackgroundRefresh()
	return usage.LoadCache(cachePath)
}

// spawnBackgroundRefresh launches a detached child process to refresh the
// usage cache, since a goroutine would be killed when this process exits.
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
// account for. Without it, badges land right at the raw terminal edge and
// get clipped/wrapped by the UI.
const rightAlignMargin = 6

// rightAlignBadge appends badge to content, padded to sit near the right
// edge of the terminal (minus rightAlignMargin for Claude Code's own
// chrome). Falls back to a single leading space when the terminal width is
// unknown or too narrow to fit both pieces.
func rightAlignBadge(content, badge string) string {
	cols, ok := terminalWidth()
	if !ok {
		return content + " " + badge
	}
	pad := cols - rightAlignMargin - visibleWidth(content) - visibleWidth(badge)
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
