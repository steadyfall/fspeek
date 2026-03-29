# fspeek

A terminal UI for browsing remote HTTP file servers. Extracts media metadata
(duration, codec, audio info) using HTTP range requests — no full downloads needed.

## Install

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
```

## Configuration

Config file: `~/.config/fspeek/config.toml`

```toml
[settings]
timeout = "5s"
max_fetches = 4

[servers.myserver]
url = "https://media.example.com/"
[servers.myserver.auth]
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
| `s` | Toggle byte sizes |
| `r` | Retry |
| `q` / `Ctrl+C` | Quit |
