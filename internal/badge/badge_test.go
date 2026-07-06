package badge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeSettings(t *testing.T, home string, enabled bool) {
	t.Helper()
	content := `{"enabledPlugins":{"caveman@caveman":false}}`
	if enabled {
		content = `{"enabledPlugins":{"caveman@caveman":true}}`
	}
	if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeScript(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, "plugins", "cache", "caveman", "caveman", "v1", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "caveman-statusline.sh")
	if err := os.WriteFile(script, []byte("#!/bin/bash\necho hi"), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func TestEnabledTrue(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, true)
	if !Enabled(home) {
		t.Error("Enabled = false, want true")
	}
}

func TestEnabledFalse(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, false)
	if Enabled(home) {
		t.Error("Enabled = true, want false")
	}
}

func TestEnabledMissingSettings(t *testing.T) {
	home := t.TempDir()
	if Enabled(home) {
		t.Error("Enabled = true, want false when settings.json is absent")
	}
}

func TestEnabledMalformedSettings(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Enabled(home) {
		t.Error("Enabled = true, want false for malformed settings.json")
	}
}

func TestFindScriptFound(t *testing.T) {
	home := t.TempDir()
	script := writeScript(t, home)

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
	writeSettings(t, home, true)
	got := Render(home, ExecRunner)
	if got != "" {
		t.Errorf("Render = %q, want empty", got)
	}
}

func TestRenderDisabledPluginSkipsScript(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, false)
	writeScript(t, home)

	got := Render(home, func(s string) (string, error) {
		t.Fatal("runner should not be called when plugin is disabled")
		return "", nil
	})
	if got != "" {
		t.Errorf("Render = %q, want empty when plugin disabled", got)
	}
}

func TestRenderUsesRunner(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, true)
	script := writeScript(t, home)

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
	writeSettings(t, home, true)
	writeScript(t, home)

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
