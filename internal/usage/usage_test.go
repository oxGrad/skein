package usage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeedsRefresh(t *testing.T) {
	cases := []struct {
		name   string
		exists bool
		age    time.Duration
		want   bool
	}{
		{"missing file", false, 0, true},
		{"fresh", true, 10 * time.Second, false},
		{"exactly at ttl", true, CacheTTL, false},
		{"stale", true, 90 * time.Second, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NeedsRefresh(c.exists, c.age); got != c.want {
				t.Errorf("NeedsRefresh(%v, %v) = %v, want %v", c.exists, c.age, got, c.want)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	body := []byte(`{"five_hour":{"utilization":42.9},"seven_day":{"utilization":7.1}}`)
	p, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if p.FiveHour == nil || *p.FiveHour != 42 {
		t.Errorf("FiveHour = %v, want 42", p.FiveHour)
	}
	if p.Week == nil || *p.Week != 7 {
		t.Errorf("Week = %v, want 7", p.Week)
	}
}

func TestParseResponseInvalidJSON(t *testing.T) {
	if _, err := ParseResponse([]byte("not json")); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("anthropic-beta") == "" {
			t.Error("missing anthropic-beta header")
		}
		w.Write([]byte(`{"five_hour":{"utilization":15},"seven_day":{"utilization":3}}`))
	}))
	defer srv.Close()

	orig := usageURL
	usageURL = srv.URL
	defer func() { usageURL = orig }()

	p, err := Fetch(srv.Client(), "test-token")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if p.FiveHour == nil || *p.FiveHour != 15 {
		t.Errorf("FiveHour = %v, want 15", p.FiveHour)
	}
	if p.Week == nil || *p.Week != 3 {
		t.Errorf("Week = %v, want 3", p.Week)
	}
}

func TestFetchRequestError(t *testing.T) {
	orig := usageURL
	usageURL = "http://127.0.0.1:0"
	defer func() { usageURL = orig }()

	if _, err := Fetch(http.DefaultClient, "tok"); err == nil {
		t.Error("expected error for unreachable server, got nil")
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	fh, wk := 33, 4
	want := Plan{FiveHour: &fh, Week: &wk}
	if err := SaveCache(path, want); err != nil {
		t.Fatalf("SaveCache error: %v", err)
	}

	got, ok := LoadCache(path)
	if !ok {
		t.Fatal("LoadCache returned ok=false")
	}
	if *got.FiveHour != fh || *got.Week != wk {
		t.Errorf("LoadCache = %+v, want FiveHour=%d Week=%d", got, fh, wk)
	}
}

func TestLoadCacheMissing(t *testing.T) {
	if _, ok := LoadCache("/nonexistent/path/cache.json"); ok {
		t.Error("LoadCache on missing file should return ok=false")
	}
}

func TestLoadCacheMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadCache(path); ok {
		t.Error("LoadCache on malformed file should return ok=false")
	}
}

func TestLoadToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	if err := os.WriteFile(path, []byte(`{"claudeAiOauth":{"accessToken":"secret-tok"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LoadToken(path); got != "secret-tok" {
		t.Errorf("LoadToken = %q, want %q", got, "secret-tok")
	}
}

func TestLoadTokenMissingOrMalformed(t *testing.T) {
	if got := LoadToken("/nonexistent/creds.json"); got != "" {
		t.Errorf("LoadToken(missing) = %q, want empty", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0o600)
	if got := LoadToken(path); got != "" {
		t.Errorf("LoadToken(malformed) = %q, want empty", got)
	}
}
