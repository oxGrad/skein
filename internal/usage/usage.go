// Package usage fetches and caches Claude plan usage (5h/weekly) from the
// OAuth usage endpoint, mirroring the shell statusline's 60s file cache.
package usage

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

// usageURL is the OAuth usage endpoint. Overridable in tests.
var usageURL = "https://api.anthropic.com/api/oauth/usage"

// CacheTTL is how long a cached usage response is considered fresh.
const CacheTTL = 60 * time.Second

// Plan holds the utilization percentages the statusline displays.
type Plan struct {
	FiveHour *int `json:"five_hour_pct,omitempty"`
	Week     *int `json:"week_pct,omitempty"`
}

type rawResponse struct {
	FiveHour struct {
		Utilization float64 `json:"utilization"`
	} `json:"five_hour"`
	SevenDay struct {
		Utilization float64 `json:"utilization"`
	} `json:"seven_day"`
}

// NeedsRefresh reports whether a cache file of the given age (or absence,
// signaled by exists=false) should be refreshed.
func NeedsRefresh(exists bool, age time.Duration) bool {
	if !exists {
		return true
	}
	return age > CacheTTL
}

// ParseResponse decodes the OAuth usage endpoint's JSON body into a Plan.
func ParseResponse(body []byte) (Plan, error) {
	var raw rawResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return Plan{}, err
	}
	fh := int(raw.FiveHour.Utilization)
	wk := int(raw.SevenDay.Utilization)
	return Plan{FiveHour: &fh, Week: &wk}, nil
}

// Fetch calls the OAuth usage endpoint with the given bearer token and
// returns the parsed plan usage.
func Fetch(client *http.Client, token string) (Plan, error) {
	req, err := http.NewRequest(http.MethodGet, usageURL, nil)
	if err != nil {
		return Plan{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := client.Do(req)
	if err != nil {
		return Plan{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Plan{}, err
	}
	return ParseResponse(body)
}

// LoadCache reads a Plan from a cache file on disk. The second return value
// is false if the file doesn't exist or can't be parsed.
func LoadCache(path string) (Plan, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, false
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return Plan{}, false
	}
	return p, true
}

// SaveCache writes a Plan to a cache file on disk as JSON.
func SaveCache(path string, p Plan) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

type credentials struct {
	ClaudeAiOauth struct {
		AccessToken string `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

// LoadToken extracts the OAuth access token from a Claude Code credentials
// file. Returns "" if the file is missing, malformed, or has no token.
func LoadToken(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var c credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return ""
	}
	return c.ClaudeAiOauth.AccessToken
}
