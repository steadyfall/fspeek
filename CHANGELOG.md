# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

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
