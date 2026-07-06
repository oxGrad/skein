// Package context computes context-window usage from a Claude Code transcript.
package context

import (
	"bufio"
	"encoding/json"
	"io"
)

// Usage mirrors the token-usage fields on a transcript line's message.usage.
type usage struct {
	InputTokens         int `json:"input_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
}

type transcriptLine struct {
	Message struct {
		Usage *usage `json:"usage"`
	} `json:"message"`
}

// tailWindow bounds how far back LastUsedTokens looks, matching the
// original script's `tac transcript | head -200`.
const tailWindow = 200

// LastUsedTokens scans the last tailWindow lines of a transcript (one JSON
// object per line, as written by Claude Code) and returns the total token
// count from the most recent line that has a message.usage block. This
// matches `tac transcript | head -200 | jq ... | head -1` in the original
// shell script.
func LastUsedTokens(r io.Reader) int {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	tail := make([]string, 0, tailWindow)
	for scanner.Scan() {
		line := scanner.Text()
		if len(tail) == tailWindow {
			tail = tail[1:]
		}
		tail = append(tail, line)
	}

	used := 0
	for i := len(tail) - 1; i >= 0; i-- {
		var line transcriptLine
		if err := json.Unmarshal([]byte(tail[i]), &line); err != nil {
			continue
		}
		if line.Message.Usage == nil {
			continue
		}
		u := line.Message.Usage
		return u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens
	}
	return used
}

// Percent computes the context-window usage percentage for a token count
// against the given window size (e.g. 200000).
func Percent(used, window int) int {
	if window <= 0 {
		return 0
	}
	return used * 100 / window
}
