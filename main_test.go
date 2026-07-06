package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeHomeUsesEnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_HOME", "/tmp/fake-claude-home")
	if got := claudeHome(); got != "/tmp/fake-claude-home" {
		t.Errorf("claudeHome() = %q, want /tmp/fake-claude-home", got)
	}
}

func TestClaudeHomeDefaultsUnderUserHome(t *testing.T) {
	t.Setenv("CLAUDE_HOME", "")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".claude")
	if got := claudeHome(); got != want {
		t.Errorf("claudeHome() = %q, want %q", got, want)
	}
}

func TestStdinPayloadParsesDocumentedFields(t *testing.T) {
	raw := `{
		"model": {"display_name": "Sonnet 5"},
		"transcript_path": "/tmp/t.jsonl",
		"context_window": {"used_percentage": 42.7},
		"rate_limits": {
			"five_hour": {"used_percentage": 23.5, "resets_at": 1738425600},
			"seven_day": {"used_percentage": 41.2, "resets_at": 1738857600}
		}
	}`
	var p stdinPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Model.DisplayName != "Sonnet 5" {
		t.Errorf("model = %q", p.Model.DisplayName)
	}
	if p.ContextWindow.UsedPercentage == nil || *p.ContextWindow.UsedPercentage != 42.7 {
		t.Errorf("used_percentage = %v, want 42.7", p.ContextWindow.UsedPercentage)
	}
	if p.RateLimits.FiveHour == nil || p.RateLimits.FiveHour.UsedPercentage != 23.5 {
		t.Errorf("five_hour = %+v", p.RateLimits.FiveHour)
	}
	if p.RateLimits.SevenDay == nil || p.RateLimits.SevenDay.ResetsAt != 1738857600 {
		t.Errorf("seven_day = %+v", p.RateLimits.SevenDay)
	}
}

func TestResolveCtxPctPrefersStdin(t *testing.T) {
	pct := 37.9
	var p stdinPayload
	p.ContextWindow.UsedPercentage = &pct
	p.TranscriptPath = "/nonexistent/should-not-be-read.jsonl"
	if got := resolveCtxPct(p); got != 37 {
		t.Errorf("resolveCtxPct = %d, want 37", got)
	}
}

func TestResolveCtxPctFallsBackToTranscript(t *testing.T) {
	dir := t.TempDir()
	tp := filepath.Join(dir, "t.jsonl")
	// 100000 tokens of 200000 fallback window -> 50%
	os.WriteFile(tp, []byte(`{"message":{"usage":{"input_tokens":100000}}}`), 0o644)

	var p stdinPayload
	p.TranscriptPath = tp
	if got := resolveCtxPct(p); got != 50 {
		t.Errorf("resolveCtxPct = %d, want 50", got)
	}
}

func TestResolveCtxPctNoData(t *testing.T) {
	var p stdinPayload
	if got := resolveCtxPct(p); got != 0 {
		t.Errorf("resolveCtxPct = %d, want 0", got)
	}
}

func TestResolvePlanPrefersStdinRateLimits(t *testing.T) {
	now := time.Unix(1738420000, 0)
	var p stdinPayload
	p.RateLimits.FiveHour = &rateWindow{UsedPercentage: 23.5, ResetsAt: 1738425600}
	p.RateLimits.SevenDay = &rateWindow{UsedPercentage: 41.2, ResetsAt: 1738857600}

	plan := resolvePlan(p, t.TempDir(), now)
	if plan == nil {
		t.Fatal("plan = nil")
	}
	if plan.fiveHour == nil || *plan.fiveHour != 23 {
		t.Errorf("fiveHour = %v, want 23", plan.fiveHour)
	}
	if plan.week == nil || *plan.week != 41 {
		t.Errorf("week = %v, want 41", plan.week)
	}
	if plan.fiveReset != "1h33m" {
		t.Errorf("fiveReset = %q, want 1h33m", plan.fiveReset)
	}
	if plan.stale {
		t.Error("stdin-sourced plan must never be stale")
	}
}

func TestResolvePlanPartialRateLimits(t *testing.T) {
	var p stdinPayload
	p.RateLimits.FiveHour = &rateWindow{UsedPercentage: 10, ResetsAt: 0}
	plan := resolvePlan(p, t.TempDir(), time.Now())
	if plan == nil || plan.fiveHour == nil {
		t.Fatal("want five-hour data")
	}
	if plan.week != nil {
		t.Errorf("week = %v, want nil when seven_day absent", plan.week)
	}
	if plan.fiveReset != "" {
		t.Errorf("fiveReset = %q, want empty for past/zero reset", plan.fiveReset)
	}
}

func TestResolvePlanFallsBackToCache(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".usage-cache.json"), []byte(`{"five_hour_pct":10,"week_pct":2}`), 0o644)

	var p stdinPayload
	plan := resolvePlan(p, home, time.Now())
	if plan == nil || plan.fiveHour == nil || *plan.fiveHour != 10 {
		t.Fatalf("plan = %+v, want cached five_hour=10", plan)
	}
	if plan.stale {
		t.Error("fresh cache must not be stale")
	}
}

func TestResolvePlanStaleCache(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, ".usage-cache.json")
	os.WriteFile(cache, []byte(`{"five_hour_pct":10,"week_pct":2}`), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(cache, old, old)

	var p stdinPayload
	plan := resolvePlan(p, home, time.Now())
	if plan == nil {
		t.Fatal("plan = nil")
	}
	if !plan.stale {
		t.Error("hour-old cache must be stale")
	}
}

func TestResolvePlanNoData(t *testing.T) {
	var p stdinPayload
	if plan := resolvePlan(p, t.TempDir(), time.Now()); plan != nil {
		t.Errorf("plan = %+v, want nil with no stdin data and no cache", plan)
	}
}

func TestFormatUntil(t *testing.T) {
	now := int64(1000000)
	cases := []struct {
		delta int64
		want  string
	}{
		{-10, ""},
		{0, ""},
		{30, "<1m"},
		{45 * 60, "45m"},
		{3600 + 23*60, "1h23m"},
		{2 * 86400, "2d"},
		{3*86400 + 4*3600, "3d4h"},
	}
	for _, c := range cases {
		if got := formatUntil(now, now+c.delta); got != c.want {
			t.Errorf("formatUntil(+%ds) = %q, want %q", c.delta, got, c.want)
		}
	}
}

var fullLayout = layout{ctxWidth: maxBarWidth, planWidth: maxBarWidth, show5h: true, showWk: true, show5hReset: true, showWkReset: true}

func TestBuildStatuslineFullLayout(t *testing.T) {
	fh, wk := 57, 46
	plan := &planInfo{fiveHour: &fh, week: &wk, fiveReset: "1h23m", weekReset: "3d4h"}
	got := buildStatusline("Sonnet 5", 33, plan, fullLayout)
	vis := stripANSI(got)

	for _, want := range []string{"Sonnet 5", "ctx", "5h", "wk", "1h23m", "3d4h", "33%", "57%", "46%"} {
		if !strings.Contains(vis, want) {
			t.Errorf("visible statusline %q missing %q", vis, want)
		}
	}
}

func TestBuildStatuslineDegradedLayouts(t *testing.T) {
	fh, wk := 57, 46
	plan := &planInfo{fiveHour: &fh, week: &wk, fiveReset: "1h23m", weekReset: "3d4h"}

	noWkReset := stripANSI(buildStatusline("Sonnet 5", 33, plan, layout{ctxWidth: 10, planWidth: 10, show5h: true, showWk: true, show5hReset: true}))
	if strings.Contains(noWkReset, "3d4h") {
		t.Errorf("no-wk-reset layout %q should drop wk countdown", noWkReset)
	}
	if !strings.Contains(noWkReset, "1h23m") {
		t.Errorf("no-wk-reset layout %q must keep 5h countdown", noWkReset)
	}

	noWk := stripANSI(buildStatusline("Sonnet 5", 33, plan, layout{ctxWidth: 10, planWidth: 10, show5h: true, show5hReset: true}))
	if strings.Contains(noWk, "wk") {
		t.Errorf("no-wk layout %q should drop wk", noWk)
	}
	if !strings.Contains(noWk, "1h23m") {
		t.Errorf("no-wk layout %q must keep 5h countdown", noWk)
	}

	narrow := stripANSI(buildStatusline("Sonnet 5", 33, plan, layout{ctxWidth: minBarWidth, planWidth: minBarWidth, show5h: true, show5hReset: true}))
	if !strings.Contains(narrow, "1h23m") {
		t.Errorf("min-width layout %q must keep 5h countdown even when narrow", narrow)
	}

	floor := stripANSI(buildStatusline("Sonnet 5", 33, plan, layout{ctxWidth: minBarWidth}))
	if strings.Contains(floor, "5h") || strings.Contains(floor, "wk") {
		t.Errorf("floor layout %q should drop all plan bars", floor)
	}
	if !strings.Contains(floor, "ctx") {
		t.Errorf("floor layout %q must keep ctx", floor)
	}
}

func TestBuildStatuslineStaleMarker(t *testing.T) {
	fh := 57
	plan := &planInfo{fiveHour: &fh, stale: true}
	vis := stripANSI(buildStatusline("", 10, plan, fullLayout))
	if !strings.Contains(vis, "5h?") {
		t.Errorf("stale plan statusline %q missing 5h? marker", vis)
	}
}

func TestBuildStatuslineNoModelNoPlan(t *testing.T) {
	vis := stripANSI(buildStatusline("", 5, nil, fullLayout))
	if !strings.HasPrefix(vis, "ctx") {
		t.Errorf("statusline %q should start with ctx when model absent", vis)
	}
	if strings.Contains(vis, "│") {
		t.Errorf("statusline %q should have no separator without model/plan", vis)
	}
}

func TestChooseLayoutNoColumnsUsesFullLayout(t *testing.T) {
	fh, wk := 57, 46
	plan := &planInfo{fiveHour: &fh, week: &wk, fiveReset: "1h23m", weekReset: "3d4h"}
	content, l := chooseLayout("Sonnet 5", 33, plan, "[CAVE]", 0, false)
	if l.ctxWidth != maxBarWidth || !l.showWk {
		t.Errorf("chooseLayout without cols = %+v, want full/max layout", l)
	}
	if !strings.Contains(stripANSI(content), "wk") {
		t.Errorf("content %q should include wk when cols unknown", content)
	}
}

func TestChooseLayoutShrinksBarsBeforeDroppingSections(t *testing.T) {
	fh, wk := 57, 46
	plan := &planInfo{fiveHour: &fh, week: &wk, fiveReset: "1h23m", weekReset: "3d4h"}

	// Full layout (max bar width) needs 68 cols; give less so it must shrink
	// bar widths, but still enough to keep wk without dropping it.
	content, l := chooseLayout("Sonnet 5", 33, plan, "", 60, true)
	if l.ctxWidth == maxBarWidth {
		t.Errorf("chooseLayout(80 cols) kept max bar width %+v, want shrunk", l)
	}
	if !l.showWk {
		t.Errorf("chooseLayout(80 cols) dropped wk %+v, want bars to shrink first", l)
	}
	if visibleWidth(content) > 80 {
		t.Errorf("content %q wider than 80 cols budget", content)
	}
}

func TestChooseLayoutDropsWkOnlyWhenMinWidthInsufficient(t *testing.T) {
	fh, wk := 57, 46
	plan := &planInfo{fiveHour: &fh, week: &wk, fiveReset: "1h23m", weekReset: "3d4h"}

	_, l := chooseLayout("Sonnet 5", 33, plan, "", 50, true)
	if l.showWk {
		t.Errorf("chooseLayout(50 cols) = %+v, want wk dropped at this width", l)
	}
}

func TestChooseLayoutLongModelNameDropsWkEarlier(t *testing.T) {
	fh, wk := 57, 46
	plan := &planInfo{fiveHour: &fh, week: &wk, fiveReset: "1h23m", weekReset: "3d4h"}

	_, shortModel := chooseLayout("A", 33, plan, "", 70, true)
	_, longModel := chooseLayout("A Very Long Model Name Indeed", 33, plan, "", 70, true)

	if !shortModel.showWk {
		t.Errorf("short model name at 70 cols dropped wk unexpectedly: %+v", shortModel)
	}
	if longModel.showWk {
		t.Errorf("long model name at 70 cols kept wk %+v, want it dropped for space", longModel)
	}
}

func TestChooseLayoutFloorWhenNothingFits(t *testing.T) {
	plan := &planInfo{}
	content, l := chooseLayout("Sonnet 5", 33, plan, "[CAVE]", 5, true)
	if l.ctxWidth != minBarWidth {
		t.Errorf("chooseLayout(5 cols) = %+v, want floor min width even though it won't fit", l)
	}
	if content == "" {
		t.Error("chooseLayout must still return content, not blank out, when nothing fits")
	}
}

func TestShortenBadge(t *testing.T) {
	cases := map[string]string{
		"\033[38;5;172m[CAVEMAN]\033[0m":       "\033[38;5;172m[CAVE]\033[0m",
		"\033[38;5;172m[CAVEMAN:ULTRA]\033[0m": "\033[38;5;172m[CAVE:ULTRA]\033[0m",
		"":                                     "",
	}
	for in, want := range cases {
		if got := shortenBadge(in); got != want {
			t.Errorf("shortenBadge(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVisibleWidth(t *testing.T) {
	cases := map[string]int{
		"abc":                           3,
		"\033[38;5;250mabc\033[0m":      3,
		"":                              0,
		"\033[1m\033[38;5;232mX\033[0m": 1,
	}
	for in, want := range cases {
		if got := visibleWidth(in); got != want {
			t.Errorf("visibleWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTerminalWidth(t *testing.T) {
	t.Setenv("COLUMNS", "120")
	if n, ok := terminalWidth(); !ok || n != 120 {
		t.Errorf("terminalWidth() = %d, %v, want 120, true", n, ok)
	}

	t.Setenv("COLUMNS", "")
	if _, ok := terminalWidth(); ok {
		t.Error("terminalWidth() ok=true for empty COLUMNS, want false")
	}

	t.Setenv("COLUMNS", "not-a-number")
	if _, ok := terminalWidth(); ok {
		t.Error("terminalWidth() ok=true for garbage COLUMNS, want false")
	}

	t.Setenv("COLUMNS", "-5")
	if _, ok := terminalWidth(); ok {
		t.Error("terminalWidth() ok=true for negative COLUMNS, want false")
	}
}

func TestRightAlignMarginOverride(t *testing.T) {
	t.Setenv("SKEIN_MARGIN", "")
	if got := rightAlignMargin(); got != 6 {
		t.Errorf("rightAlignMargin() = %d, want default 6", got)
	}
	t.Setenv("SKEIN_MARGIN", "12")
	if got := rightAlignMargin(); got != 12 {
		t.Errorf("rightAlignMargin() = %d, want 12", got)
	}
	t.Setenv("SKEIN_MARGIN", "junk")
	if got := rightAlignMargin(); got != 6 {
		t.Errorf("rightAlignMargin() = %d, want default 6 for junk", got)
	}
}

func TestRightAlignBadgeUnknownWidth(t *testing.T) {
	t.Setenv("COLUMNS", "")
	got := rightAlignBadge("hello", "[BADGE]")
	if got != "hello [BADGE]" {
		t.Errorf("rightAlignBadge = %q, want %q", got, "hello [BADGE]")
	}
}

func TestRightAlignBadgePadsToWidth(t *testing.T) {
	t.Setenv("COLUMNS", "20")
	t.Setenv("SKEIN_MARGIN", "6")
	got := rightAlignBadge("hello", "world")
	if got != "hello    world" {
		t.Errorf("rightAlignBadge = %q, want padded to 20-margin cols", got)
	}
	if len(got) != 20-6 {
		t.Errorf("rightAlignBadge len = %d, want 14", len(got))
	}
}

func TestRightAlignBadgeTooNarrowFallsBackToOneSpace(t *testing.T) {
	t.Setenv("COLUMNS", "3")
	got := rightAlignBadge("hello", "world")
	if got != "hello world" {
		t.Errorf("rightAlignBadge = %q, want single-space fallback", got)
	}
}

func TestVersionString(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	version = "1.2.3"
	if got := versionString(); got != "1.2.3" {
		t.Errorf("versionString() = %q, want injected 1.2.3", got)
	}

	// In tests build info reports "(devel)"/empty, so the dev default holds.
	version = "dev"
	if got := versionString(); got != "dev" {
		t.Errorf("versionString() = %q, want dev fallback", got)
	}
}

func TestRefreshUsageCacheNoToken(t *testing.T) {
	dir := t.TempDir()
	refreshUsageCache(filepath.Join(dir, "cache.json"), filepath.Join(dir, "creds.json"))
	if _, err := os.Stat(filepath.Join(dir, "cache.json")); err == nil {
		t.Error("expected no cache file to be written when there's no token")
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
