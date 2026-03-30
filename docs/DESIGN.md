# Design System — fspeek

## Product Context
- **What this is:** Terminal UI for browsing remote HTTP file servers with zero-download media metadata extraction
- **Who it's for:** Developers and sysadmins who need to inspect remote media files (MP4, MKV, SRT) without downloading them
- **Space/industry:** Developer tools, terminal utilities (peers: ranger, lf, nnn — but none do media metadata)
- **Project type:** TUI application (bubbletea + lipgloss, 256-color)

## Aesthetic Direction
- **Direction:** Spectral — cold precision for navigation, warm signal for discovery
- **Decoration level:** Minimal (terminal constraints; typography and color do all the work)
- **Mood:** A media forensics tool. The left pane is a navigator; the right pane is the payoff. The design makes that distinction tangible through the warm/cool color split.

## Color Palette (256-color ANSI)

Defined in `ui/theme.go` (`SpectralTheme`). Never hard-code color values in style definitions — always reference the theme.

| Token | ANSI | Usage |
|-------|------|-------|
| `BorderColor` | 241 — dark gray | Pane borders |
| `NormalFg` | 252 — light gray | Default list text |
| `DirFg` | 39 — electric blue | Directory entries |
| `CursorFg` | 232 — near-black | Selected row text |
| `CursorBg` | 45 — bright cyan | Selected row background |
| `MetaTitleFg` | 209 — coral | Metadata pane title |
| `MetaLabelFg` | 242 — gray | Metadata field labels |
| `MetaValueFg` | 252 — light gray | Metadata field values |
| `MetaErrFg` | 203 — coral-red | Error messages in metadata pane |
| `StatusFg` | 209 — coral | Status bar text |
| `StatusBg` | 235 — very dark | Status bar background |
| `StatusErrFg` | 203 — coral-red | Error status bar text |
| `HelpFg` | 238 — dark gray | Help bar (intentionally de-emphasized) |
| `SpinnerFg` | 45 — bright cyan | Loading indicator (matches cursor) |

## Typography (terminal context)
- **Bold:** Cursor row, directory names, metadata titles
- **Italic:** Error messages in the metadata pane
- No font choices — terminal monospace applies; the user's terminal font renders all text

## Layout
- **Split:** 55% list / 45% metadata — list gets more room for long filenames
- **Border style:** `ThickBorder` on both panes
- **Border overhead:** 4 chars total (2 panes × 1 char per side); `borderOverhead = 4` in `ui/model.go`
- **Status bar:** 1 row, full width, at bottom
- **Help bar:** 1 row, full width, below status bar

## Spacing
- **Label column width:** 12 chars (`MetaLabelWidth` in theme) for fixed-width label alignment
- **Status/help bar padding:** 0 vertical, 1 horizontal

## Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-03-30 | SPECTRAL theme created | Give fspeek a visual identity that reflects its media-inspection purpose; warm/cool split across panes encodes navigation vs. discovery |
| 2026-03-30 | `ThickBorder` over `RoundedBorder` | More deliberate, less web-inspired; feels like a broadcast monitor bezel |
| 2026-03-30 | Cyan cursor (45) over blue-purple (62) | Cyan matches spinner — "this is the active item being fetched" — creates a coherent active-state signal |
| 2026-03-30 | Coral status bar (209) over gray (252) | Status bar earns visual identity; echoes the metadata title accent |
| 2026-03-30 | 55/45 layout kept | List needs room for long filenames; confirmed by user |
