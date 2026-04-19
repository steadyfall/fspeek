# fspeek

[![CI](https://github.com/steadyfall/fspeek/actions/workflows/ci.yml/badge.svg)](https://github.com/steadyfall/fspeek/actions/workflows/ci.yml)

A terminal UI for browsing remote HTTP file servers. Extracts media metadata
(duration, codec, audio info) using HTTP range requests — no full downloads needed.

## Install

**Pre-built binaries** (linux/darwin, amd64/arm64) are available on the
[Releases page](https://github.com/steadyfall/fspeek/releases).

Or install with Go:

```bash
go install github.com/steadyfall/fspeek/cmd/fspeek@latest
```

Or build from source:

```bash
git clone https://github.com/steadyfall/fspeek.git
cd fspeek
go build ./cmd/fspeek
```

## Usage

```bash
fspeek --url https://example.com/files/
fspeek --server myserver          # named server from config
fspeek --url ... --no-cache       # skip local cache
fspeek --url ... --bytes          # show sizes in bytes
fspeek --version                  # print the shipped version
```

## Configuration

Config file: `~/.config/fspeek/config.toml`

```toml
[settings]
metadata_timeout_s = 5
max_concurrent_fetches = 4

[[server]]
name = "myserver"
url = "https://media.example.com/"
[server.auth]
type = "basic"
username = "user"
password = "pass"
```

## Keys

| Key | Action |
|-----|--------|
| `j` / `↓` | Down |
| `k` / `↑` | Up |
| `l` / `→` / `Enter` | Open directory |
| `h` / `←` / `Backspace` | Go back |
| `s` | Cycle sort (Name ▲/▼, Count ▲/▼, Size ▲/▼) |
| `b` | Toggle byte sizes |
| `/` | Enter filter mode (type to narrow list, `esc` to clear) |
| `esc` | Clear filter / quit |
| `r` | Force-refresh listing |
| `q` / `Ctrl+C` | Quit |

## Project Docs

- [`docs/DESIGN.md`](docs/DESIGN.md) explains the Spectral TUI design system and theme tokens.
- [`docs/TODOS.md`](docs/TODOS.md) tracks intentionally deferred follow-up work.
