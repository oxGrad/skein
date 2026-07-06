# skein

A statusline for [Claude Code](https://code.claude.com/docs/en/statusline), written in Go.

Shows the active model, a context-window usage bar, 5-hour/weekly plan usage
bars with reset countdowns, and (if the
[caveman](https://github.com/JuliusBrussee/caveman) plugin is active) a
right-aligned mode badge.

```
Fable 5 │ ctx ███33%░░░░ │ 5h ███57%░░░░ 1h23m │ wk ███46%░░░░ 3d5h    [CAVE]
```

Data comes from the JSON Claude Code pipes to statusline scripts
(`context_window.used_percentage`, `rate_limits`); older Claude Code versions
fall back to transcript parsing and a cached OAuth usage lookup. The layout
degrades gracefully on narrow terminals - countdowns drop first, then the
weekly bar, then the 5-hour bar. Set `SKEIN_MARGIN` to adjust the right-edge
margin used when aligning the badge (default 6).

## Install

### Homebrew

```sh
brew install oxGrad/tap/skein
```

### Go

```sh
go install github.com/oxGrad/skein@latest
```

## Setup

After installing, point Claude Code's statusline at the binary:

```sh
skein install
```

This patches `statusLine` in `~/.claude/settings.json` (or `$CLAUDE_HOME/settings.json`)
to run the installed binary, leaving every other setting untouched.

## Development

```sh
just build   # build ./bin/skein
just test    # go test ./...
just install # ./bin/skein install
```
