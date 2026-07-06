package context

import (
	"strings"
	"testing"
)

func TestLastUsedTokensPicksLastMatch(t *testing.T) {
	transcript := strings.Join([]string{
		`{"message":{"usage":{"input_tokens":100,"cache_read_input_tokens":10,"cache_creation_input_tokens":0}}}`,
		`{"message":{"other":"noop"}}`,
		`{"message":{"usage":{"input_tokens":500,"cache_read_input_tokens":20,"cache_creation_input_tokens":5}}}`,
	}, "\n")

	got := LastUsedTokens(strings.NewReader(transcript))
	want := 500 + 20 + 5
	if got != want {
		t.Errorf("LastUsedTokens = %d, want %d", got, want)
	}
}

func TestLastUsedTokensNoUsage(t *testing.T) {
	transcript := `{"message":{"other":"noop"}}`
	if got := LastUsedTokens(strings.NewReader(transcript)); got != 0 {
		t.Errorf("LastUsedTokens = %d, want 0", got)
	}
}

func TestLastUsedTokensIgnoresMalformedLines(t *testing.T) {
	transcript := strings.Join([]string{
		`not json`,
		`{"message":{"usage":{"input_tokens":42}}}`,
	}, "\n")
	if got := LastUsedTokens(strings.NewReader(transcript)); got != 42 {
		t.Errorf("LastUsedTokens = %d, want 42", got)
	}
}

func TestLastUsedTokensRespectsTailWindow(t *testing.T) {
	lines := make([]string, 0, tailWindow+5)
	lines = append(lines, `{"message":{"usage":{"input_tokens":9999}}}`)
	for range tailWindow + 4 {
		lines = append(lines, `{"message":{"other":"noop"}}`)
	}
	transcript := strings.Join(lines, "\n")
	if got := LastUsedTokens(strings.NewReader(transcript)); got != 0 {
		t.Errorf("LastUsedTokens = %d, want 0 (old usage line outside tail window)", got)
	}
}

func TestPercent(t *testing.T) {
	cases := []struct{ used, window, want int }{
		{0, 200000, 0},
		{100000, 200000, 50},
		{200000, 200000, 100},
		{50000, 0, 0},
	}
	for _, c := range cases {
		if got := Percent(c.used, c.window); got != c.want {
			t.Errorf("Percent(%d, %d) = %d, want %d", c.used, c.window, got, c.want)
		}
	}
}
