package main

import (
	"os"
	"path/filepath"
	"testing"
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

func TestLoadOrRefreshUsageNoCacheNoCreds(t *testing.T) {
	dir := t.TempDir()
	plan, ok := loadOrRefreshUsage(dir)
	if ok {
		t.Errorf("expected ok=false with no creds/cache, got plan=%+v", plan)
	}
}

func TestLoadOrRefreshUsageFreshCache(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".usage-cache.json")
	if err := os.WriteFile(cachePath, []byte(`{"five_hour_pct":10,"week_pct":2}`), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, ok := loadOrRefreshUsage(dir)
	if !ok {
		t.Fatal("expected ok=true for fresh cache")
	}
	if plan.FiveHour == nil || *plan.FiveHour != 10 {
		t.Errorf("FiveHour = %v, want 10", plan.FiveHour)
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

func TestRightAlignBadgeUnknownWidth(t *testing.T) {
	t.Setenv("COLUMNS", "")
	got := rightAlignBadge("hello", "[BADGE]")
	if got != "hello [BADGE]" {
		t.Errorf("rightAlignBadge = %q, want %q", got, "hello [BADGE]")
	}
}

func TestRightAlignBadgePadsToWidth(t *testing.T) {
	t.Setenv("COLUMNS", "20")
	got := rightAlignBadge("hello", "world")
	if got != "hello    world" {
		t.Errorf("rightAlignBadge = %q, want padded to 20-margin cols", got)
	}
	if len(got) != 20-rightAlignMargin {
		t.Errorf("rightAlignBadge len = %d, want 20", len(got))
	}
}

func TestRightAlignBadgeTooNarrowFallsBackToOneSpace(t *testing.T) {
	t.Setenv("COLUMNS", "3")
	got := rightAlignBadge("hello", "world")
	if got != "hello world" {
		t.Errorf("rightAlignBadge = %q, want single-space fallback", got)
	}
}

func TestRefreshUsageCacheNoToken(t *testing.T) {
	dir := t.TempDir()
	// No credentials file present: refreshUsageCache should return without
	// writing a cache file.
	refreshUsageCache(filepath.Join(dir, "cache.json"), filepath.Join(dir, "creds.json"))
	if _, err := os.Stat(filepath.Join(dir, "cache.json")); err == nil {
		t.Error("expected no cache file to be written when there's no token")
	}
}
