# fspeek

TUI browser for remote HTTP file servers. Uses HTTP range requests to extract media metadata (MP4, MKV, SRT) without downloading full files. Two-pane Vim-keyed interface with SQLite cache, TOML config, and Basic/Bearer auth support.

## Commands

```bash
go build ./cmd/fspeek        # build binary → ./fspeek
go test ./...                # run all tests

./fspeek --url https://example.com/files/
./fspeek --server <name>     # named server from config
./fspeek --no-cache --bytes  # skip cache; show sizes in bytes
./fspeek --version
```

Default paths: config `~/.config/fspeek/config.toml`, cache DB `~/.cache/fspeek/cache.db`.

## Packages

| Package | Purpose |
|---------|---------|
| `cmd/fspeek` | CLI flags, dependency wiring, TUI launch |
| `config` | TOML loader; HTTP client builder with auth RoundTripper |
| `cache` | SQLite-backed directory listing + metadata cache (24h TTL) |
| `fetcher` | Range-based metadata extractors: MP4, MKV, SRT |
| `parser` | Directory listers: NginxJSON, GenericHref, Cascade |
| `ui` | Bubbletea TUI model, two-pane layout, debounce, prefetch |

## Conventions

- **tea.Cmd discipline** — no goroutines inside `Update()`; all async work via returned `tea.Cmd`
- **Sentinel errors** — define typed sentinels (`ErrNoMatch`, `ErrRangeUnsupported`) and wrap with `fmt.Errorf("%w: …")`
- **Interfaces at boundaries** — `DirectoryLister`, `Cache`, `MetadataFetcher` keep packages decoupled; inject via `ui.Options`
- **Nonce tracking** — stale metadata results are dropped by comparing `fetchNonce` before applying
- **Non-fatal cache init** — cache failure prints a warning and continues; never fatal
- **Context everywhere** — pass `context.Context` to all network calls; respect cancellation

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): description`

Common types: `feat`, `fix`, `chore`, `test`, `refactor`, `docs`

Examples:
- `feat(fetcher): add MP3 metadata extractor`
- `fix(cache): handle nil entry on miss`

Before every commit, run:

```bash
go fmt ./...
go vet ./...
```

## Do not

- Add CGO dependencies — SQLite uses `modernc.org/sqlite` (pure Go)
- Add goroutines inside bubbletea `Update()` — return a `tea.Cmd` instead
- Commit the `fspeek` binary

## Project docs

- `docs/DESIGN.md` documents the Spectral theme, layout decisions, and UI tokens.
- `docs/TODOS.md` tracks intentionally deferred follow-up work.

## Skill routing

When the user's request matches an available skill, ALWAYS invoke it using the Skill
tool as your FIRST action. Do NOT answer directly, do NOT use other tools first.
The skill has specialized workflows that produce better results than ad-hoc answers.

Key routing rules:
- Product ideas, "is this worth building", brainstorming → invoke office-hours
- Bugs, errors, "why is this broken", 500 errors → invoke investigate
- Ship, deploy, push, create PR → invoke ship
- QA, test the site, find bugs → invoke qa
- Code review, check my diff → invoke review
- Update docs after shipping → invoke document-release
- Weekly retro → invoke retro
- Design system, brand → invoke design-consultation
- Visual audit, design polish → invoke design-review
- Architecture review → invoke plan-eng-review
