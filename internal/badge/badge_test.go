package badge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindScriptFound(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "plugins", "cache", "caveman", "caveman", "v1.2.3", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "caveman-statusline.sh")
	if err := os.WriteFile(script, []byte("#!/bin/bash\necho hi"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := FindScript(home); got != script {
		t.Errorf("FindScript = %q, want %q", got, script)
	}
}

func TestFindScriptMissing(t *testing.T) {
	home := t.TempDir()
	if got := FindScript(home); got != "" {
		t.Errorf("FindScript = %q, want empty", got)
	}
}

func TestRenderNoScript(t *testing.T) {
	home := t.TempDir()
	got := Render(home, ExecRunner)
	if got != "" {
		t.Errorf("Render = %q, want empty", got)
	}
}

func TestRenderUsesRunner(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "plugins", "cache", "caveman", "caveman", "v1", "hooks")
	os.MkdirAll(dir, 0o755)
	script := filepath.Join(dir, "caveman-statusline.sh")
	os.WriteFile(script, []byte(""), 0o755)

	got := Render(home, func(s string) (string, error) {
		if s != script {
			t.Errorf("runner got script %q, want %q", s, script)
		}
		return "BADGE", nil
	})
	if got != "BADGE" {
		t.Errorf("Render = %q, want BADGE", got)
	}
}

func TestRenderRunnerError(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "plugins", "cache", "caveman", "caveman", "v1", "hooks")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "caveman-statusline.sh"), []byte(""), 0o755)

	got := Render(home, func(s string) (string, error) {
		return "", errors.New("boom")
	})
	if got != "" {
		t.Errorf("Render = %q, want empty on runner error", got)
	}
}

func TestExecRunnerTrimsTrailingNewline(t *testing.T) {
	home := t.TempDir()
	script := filepath.Join(home, "echo.sh")
	os.WriteFile(script, []byte("#!/bin/bash\necho hello\n"), 0o755)

	out, err := ExecRunner(script)
	if err != nil {
		t.Fatalf("ExecRunner error: %v", err)
	}
	if out != "hello" {
		t.Errorf("ExecRunner = %q, want %q", out, "hello")
	}
}
