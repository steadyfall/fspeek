# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.2.4.0] - 2026-04-22

### Changed

- **gstack upgrade v1.5.1.0 → v1.6.0.0** — upgrades the vendored AI assistant tooling. Key security additions: dual-listener HTTP architecture for `pair-agent` (the ngrok tunnel now forwards only a locked port, making `/health`, `/cookie-picker`, and `/inspector/*` physically unreachable over the tunnel), a 17-command allowlist for remote agents, HttpOnly SSE session cookies replacing URL-embedded tokens, SSRF protection on download+scrape paths, envelope sentinel escape fixes in snapshots, and hidden-element detection across all DOM-reading channels.

## [0.2.3.1] - 2026-04-20

### Changed

- **gstack upgrade v0.14.1.0 → v1.5.1.0** — upgrades the vendored AI assistant tooling. Key additions: `/make-pdf` skill (markdown to publication-quality PDF via headless Chromium), `/context-save` and `/context-restore` (session state as readable markdown), design taste memory across sessions, an 8-layer prompt injection defense stack in the sidebar agent, and fixes for PDF double-page-numbers, HTML entity double-escaping, and Linux font fallback.

## [0.2.3.0] - 2026-04-20

### Added

- **Smoke tests** — two CI smoke jobs verify the binary works after every build. The `smoke` job in `ci.yml` builds from source and checks that `--version` prints a recognizable version and that running with no `--url` exits non-zero with usage output. A new `release-smoke.yml` workflow triggers on every GitHub release publication: it enters a `golang:1.26.2-trixie` container, installs fspeek via `go install`, and runs the same checks to confirm the released module is installable and functional end-to-end.

### Changed

- No-URL smoke check now captures output on the first invocation and greps that, avoiding a redundant second binary run.
- Release smoke install step assigns `github.ref_name` to an env var before shell expansion.
- Release smoke version check no longer accepts `dev` — a release binary must print a real version string.

## [0.2.2.0] - 2026-04-19

### Added

- **Release pipeline** — goreleaser config builds pre-compiled binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 on every `v*.*.*` tag push. Release workflow publishes binaries and checksums to GitHub Releases. `go install github.com/steadyfall/fspeek@latest` now resolves to pre-built artifacts.
- **CI workflow** — runs `gofmt`, `go vet`, and `go test -race` on every push and pull request targeting `main`.
- **Supply chain hardening** — all GitHub Actions pinned to specific commit SHAs (`actions/checkout` v4.2.2, `actions/setup-go` v5.2.0, `goreleaser/goreleaser-action` v6.4.0) and goreleaser binary pinned to v2.15.3.
- **CODEOWNERS** — workflow and goreleaser config changes require `@steadyfall` review.
- **Release test gate** — release workflow runs the full test suite before invoking goreleaser, preventing untested tags from publishing binaries.

### Changed

- `fspeek --version` on dev builds (not built via goreleaser) now reports `dev` instead of a stale version string.

## [0.2.1.0] - 2026-04-01

### Added

- **Cache-first startup** — fspeek now loads directory listings from the SQLite cache on startup instead of always fetching from the server first. The first view appears immediately if the cache is warm; HTTP fetch only fires on a cache miss.
- `applyFromCache()` internal helper deduplicates cache-hit state setup across startup and directory navigation, and respects the active sort order.

### Changed

- Pressing `r` now force-refreshes the listing from HTTP at any time (not just when an error is shown), invalidating both the current directory listing and the selected file's metadata in the cache.
- Help bar updated: "retry" → "refresh".

### Fixed

- `applyFromCache` now has an internal nil guard on the cache reference, making it safe against callers that don't gate on nil externally.
- `r` no longer incorrectly deletes the cached listing of a child directory when the cursor is on a directory entry.
- Loading spinner now appears during the initial HTTP fetch (was missing when no cache hit on startup).

## [0.2.0.0] - 2026-03-31

### Added

- **Columnar directory layout** — the file list now shows NAME, COUNT, and SIZE in aligned columns with a stable header row. Column widths are computed from all entries so the layout stays stable while typing a filter.
- **6-state sort cycle** — press `s` to cycle through Name ▲, Count ▲, Size ▲, Name ▼, Count ▼, Size ▼. The active sort column shows a ▲/▼ indicator in the header.
- **Filter mode** — press `/` to enter filter mode; type to narrow the list to matching filenames (case-insensitive). `esc` clears the filter. `backspace` deletes the last character. Arrow keys and `enter` work while filtering.
- **Partial-cache indicator** — directories whose size data comes from an incomplete cache scan show a trailing `~` in the size column, styled in amber.

### Changed

- Help bar updated: now shows `h/backspace/← back`, `esc exit`, and `/  filter`.
- The COUNT column is omitted entirely when no directory size data is available, preventing SIZE column misalignment in file-only listings.
- Sort state persists across directory navigation.
- `truncate()` now uses display-width-aware truncation (`runewidth`) for correct behavior with CJK and other double-width filenames.
- Sort cycle uses a named sentinel constant (`sortByNumStates`) instead of a magic `% 6` — adding future sort modes won't require updating the cycle.

### Fixed

- `esc` in normal mode now quits as advertised in the help bar (was a no-op).
- `prefetchNext` now iterates visible entries (respecting active filter) instead of all entries — previously it queued wrong entries for background prefetch when a filter was active.
- Column layout height guard: `entryHeight` is now clamped to at least 1 after header reservation, preventing broken windowing at very small terminal heights.

### Added (tests)

- 32 new tests covering columnar layout, sort cycle (ascending + descending + nil-DirSize sentinel), filter mode (typing, backspace, esc, navigation passthrough, case-insensitive match), help text, and header sort indicators.
- Regression tests for `esc` quit in normal mode, `esc` clear-filter in filter mode, and `prefetchNext` filter correctness.

## [0.1.1.0] - 2026-03-30

### Added

- Cursor position persisted per directory URL across the session; revisiting a directory restores the last-used cursor position, clamped to the current entry count (`ui/model.go`)

### Changed

- fspeek now uses the Spectral theme, with thicker pane borders, a stronger status bar, and clearer contrast between the file list and metadata pane.
- File sizes and directory counts are now dimmed so names are easier to scan in the list.

### Fixed

- Cursor position for a previously-visited directory was silently clobbered when the user backed out before an in-flight listing arrived; saved cursor is now preserved by skipping the write when `loadingListing` is true (`ui/model.go:273`)
- Selected directory rows now stay readable instead of inheriting conflicting colors.
- Directory rows no longer render with a double trailing slash.

## [0.1.0.0] - 2026-03-29

### Added

- TUI two-pane file browser with Vim keybindings (`h/j/k/l`, `enter`, `backspace`, `q`, `r`, `s`) powered by Bubbletea and lipgloss (`ui/`)
- MP4 metadata extraction via HTTP range requests — duration, video codec, resolution, audio codec (`fetcher/mp4.go`)
- MKV/WebM metadata extraction via EBML range parsing — duration, video track dimensions, audio codec (`fetcher/mkv.go`)
- SRT subtitle metadata extraction — duration from last timestamp, subtitle count (`fetcher/srt.go`)
- `DirectoryLister` interface with three implementations: NginxJSON autoindex, generic href scraper, standard nginx HTML autoindex, and a `Cascade` fallback combinator (`parser/`)
- SQLite-backed directory listing and metadata cache with 24 h TTL, `modernc.org/sqlite` (pure Go, no CGO) (`cache/`)
- TOML config loader supporting `[settings]` (timeout, max fetches, cache TTL, bytes display) and `[servers.<name>]` named servers with Basic/Bearer auth (`config/`)
- Authenticated `http.Client` builder with custom `RoundTripper` for Basic and Bearer auth (`config/config.go`)
- CLI entry point with `--url`, `--server`, `--no-cache`, `--bytes`, `--version` flags and cross-host credential-leak guard (`cmd/fspeek/`)
- Nonce-based stale fetch protection: metadata results are dropped if `fetchNonce` no longer matches the selected file
- Semaphore-bounded concurrent metadata prefetch (configurable via `max_fetches`)
- Debounced metadata fetch on cursor move (100 ms) to avoid fetching on rapid scrolling
- URL percent-encoding in `joinURL` to handle filenames with spaces and special characters

### Fixed

- `io.ReadAll` on HTTP 206 responses is now capped to `end-start+1` bytes via `io.LimitReader` to prevent OOM if a server ignores the Range end byte and streams the full file (`fetcher/range.go`)
- Retry key `r` now batches `spinnerCmd()` alongside the fetch cmd so the spinner animates during the refetch instead of freezing on the last frame (`ui/model.go`)
- Cache `Invalidate` deletes are wrapped in a transaction for atomicity (`cache/sqlite.go`)
- `CacheTTLHours` config value is now wired to `SQLiteCache` TTL instead of being ignored (`cache/sqlite.go`, `cmd/fspeek/main.go`)
- `navigateTo` no longer re-pushes the current URL onto history when navigating back, preventing duplicate history entries (`ui/model.go`)
- `prefetchNext` closures now capture the `cache` reference correctly so metadata is actually stored (`ui/model.go`)
- `loadingListing` and `listingErr` are cleared on cached-hit navigation to avoid showing stale error state (`ui/model.go`)
- Cross-host `--server` + `--url` combination is now rejected to prevent credential leak to attacker-supplied URL (`cmd/fspeek/main.go`)
- `joinURL` now percent-encodes path segments to prevent URL corruption on filenames with spaces (`parser/`)
- `Canonicalize` in the SQLite cache no longer double-encodes already-percent-encoded URLs (`cache/sqlite.go`)

### Added (tests)

- Full test suite for all packages: `cache`, `config`, `fetcher`, `parser`, `ui`
- Regression tests for `joinURL` encoding and cached-nav state machine
- QA-driven coverage additions: `Dispatch` Accept-Ranges branches, `NewCascade`/`cascadeLister.List`, `formatDirSize` partial branch
