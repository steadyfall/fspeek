// Package ui implements the fspeek terminal user interface using bubbletea.
package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"golang.org/x/sync/semaphore"

	"github.com/steadyfall/fspeek/cache"
	"github.com/steadyfall/fspeek/fetcher"
	"github.com/steadyfall/fspeek/parser"

	"net/http"
)

// spinnerFrames is the animation sequence for the loading indicator.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// --- Messages ---

// listingMsg carries the result of a directory listing fetch.
type listingMsg struct {
	url     string
	entries []cache.Entry
	err     error
}

// metadataMsg carries the result of a metadata fetch.
type metadataMsg struct {
	nonce string // URL of the file; used to discard stale results
	meta  *fetcher.Metadata
	err   error
}

// debounceMsg fires after the 150ms debounce timer.
type debounceMsg struct{ nonce string }

// spinnerTickMsg drives the spinner animation.
type spinnerTickMsg struct{}

// Model is the bubbletea model for fspeek.
type Model struct {
	// Directory state.
	baseURL string
	entries []cache.Entry
	cursor  int
	history []string // stack of parent URLs for navigation back

	// Listing fetch state.
	loadingListing bool
	listingErr     error

	// Metadata fetch state.
	metadata   *fetcher.Metadata
	metaErr    error
	fetching   bool
	fetchNonce string // URL of in-flight/last-requested fetch; discard stale results
	cancel     context.CancelFunc

	// Prefetch tracking (URLs we've already launched a fetch for).
	prefetched map[string]bool

	// UI settings.
	showBytes bool
	width     int
	height    int
	spinFrame int

	// Dependencies (injected).
	cache    cache.Cache
	client   *http.Client
	lister   parser.DirectoryLister
	fetchers []fetcher.MetadataFetcher
	sem      *semaphore.Weighted
}

// Options configures the Model.
type Options struct {
	Cache      cache.Cache
	Client     *http.Client
	Lister     parser.DirectoryLister
	Fetchers   []fetcher.MetadataFetcher
	MaxFetches int
	ShowBytes  bool
}

// New creates an initialised Model for the given root URL.
func New(rootURL string, opts Options) Model {
	maxFetches := opts.MaxFetches
	if maxFetches <= 0 {
		maxFetches = 4
	}
	return Model{
		baseURL:    rootURL,
		history:    []string{},
		prefetched: map[string]bool{},
		showBytes:  opts.ShowBytes,
		cache:      opts.Cache,
		client:     opts.Client,
		lister:     opts.Lister,
		fetchers:   opts.Fetchers,
		sem:        semaphore.NewWeighted(int64(maxFetches)),
		width:      120,
		height:     30,
	}
}

// Init issues the initial directory listing fetch.
func (m Model) Init() tea.Cmd {
	return fetchListingCmd(m.baseURL, m.cache, m.client, m.lister)
}

// Update handles all messages and key events (tea.Cmd discipline: no goroutines here).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	// --- Directory listing result ---
	case listingMsg:
		if msg.url != m.baseURL {
			return m, nil // stale
		}
		m.loadingListing = false
		if msg.err != nil {
			m.listingErr = msg.err
			return m, nil
		}
		m.listingErr = nil
		m.entries = msg.entries
		m.cursor = 0
		m.metadata = nil
		m.metaErr = nil
		m.fetchNonce = ""
		m.prefetched = map[string]bool{}
		// Save to cache.
		if m.cache != nil {
			_ = m.cache.SetListing(m.baseURL, m.entries, "")
		}
		// Populate DirSize from cache for subdirs.
		if m.cache != nil {
			for i, e := range m.entries {
				if e.IsDir {
					m.entries[i].DirSize = m.cache.ComputeDirSize(e.URL)
				}
			}
		}
		return m, m.debounceMetaCmd()

	// --- Debounce timer fired ---
	case debounceMsg:
		if msg.nonce != m.fetchNonce {
			return m, nil // stale
		}
		return m, m.launchMetaFetchCmd()

	// --- Metadata result ---
	case metadataMsg:
		if msg.nonce != m.fetchNonce {
			return m, nil // stale
		}
		m.fetching = false
		m.metadata = msg.meta
		m.metaErr = msg.err
		// Cache the result.
		if m.cache != nil && msg.err == nil && msg.meta != nil {
			_ = m.cache.SetMetadata(m.fetchNonce, msg.meta, "")
		}
		return m, m.prefetchNext()

	// --- Spinner tick ---
	case spinnerTickMsg:
		if m.fetching || m.loadingListing {
			m.spinFrame = (m.spinFrame + 1) % len(spinnerFrames)
			return m, spinnerCmd()
		}
		return m, nil

	// --- Key events ---
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			return m, m.moveCursor()
		}

	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			return m, m.moveCursor()
		}

	case "l", "enter", "right":
		if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			if e.IsDir {
				return m.navigateTo(e.URL, true)
			}
		}

	case "h", "backspace", "left":
		if len(m.history) > 0 {
			parent := m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			return m.navigateTo(parent, false)
		}

	case "s":
		m.showBytes = !m.showBytes

	case "r":
		if m.listingErr != nil {
			m.loadingListing = true
			return m, tea.Batch(
				fetchListingCmd(m.baseURL, m.cache, m.client, m.lister),
				spinnerCmd(),
			)
		}
	}
	return m, nil
}

// moveCursor cancels any in-flight fetch and starts the debounce timer.
func (m *Model) moveCursor() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.fetching = false
	m.metadata = nil
	m.metaErr = nil
	m.fetchNonce = m.currentURL()
	return m.debounceMetaCmd()
}

// navigateTo switches the current directory.
func (m Model) navigateTo(url string, pushHistory bool) (tea.Model, tea.Cmd) {
	// Try cache first.
	if m.cache != nil {
		if entries, _, err := m.cache.GetListing(url); err == nil {
			if pushHistory {
				m.history = append(m.history, m.baseURL)
			}
			m.baseURL = url
			m.entries = entries
			m.cursor = 0
			m.loadingListing = false
			m.listingErr = nil
			m.metadata = nil
			m.metaErr = nil
			m.fetchNonce = ""
			m.prefetched = map[string]bool{}
			for i, e := range m.entries {
				if e.IsDir {
					m.entries[i].DirSize = m.cache.ComputeDirSize(e.URL)
				}
			}
			return m, m.debounceMetaCmd()
		}
	}
	// Cache miss: fetch from server.
	if pushHistory {
		m.history = append(m.history, m.baseURL)
	}
	m.baseURL = url
	m.entries = nil
	m.loadingListing = true
	m.listingErr = nil
	m.metadata = nil
	m.metaErr = nil
	m.prefetched = map[string]bool{}
	return m, tea.Batch(
		fetchListingCmd(url, m.cache, m.client, m.lister),
		spinnerCmd(),
	)
}

// currentURL returns the URL of the currently selected entry, or "".
func (m *Model) currentURL() string {
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		return ""
	}
	return m.entries[m.cursor].URL
}

// debounceMetaCmd starts the 150ms debounce timer for the current entry.
func (m *Model) debounceMetaCmd() tea.Cmd {
	nonce := m.currentURL()
	if nonce == "" {
		return nil
	}
	entry := m.entries[m.cursor]
	if entry.IsDir {
		return nil
	}
	m.fetchNonce = nonce
	return func() tea.Msg {
		time.Sleep(150 * time.Millisecond)
		return debounceMsg{nonce: nonce}
	}
}

// launchMetaFetchCmd launches the actual metadata fetch as a tea.Cmd.
func (m *Model) launchMetaFetchCmd() tea.Cmd {
	url := m.fetchNonce
	if url == "" {
		return nil
	}
	// Check cache first.
	if m.cache != nil {
		if meta, _, err := m.cache.GetMetadata(url); err == nil {
			return func() tea.Msg {
				return metadataMsg{nonce: url, meta: meta}
			}
		}
	}
	m.fetching = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.cancel = cancel

	client := m.client
	fetchers := m.fetchers
	sem := m.sem

	return tea.Batch(
		spinnerCmd(),
		func() tea.Msg {
			defer cancel()
			if err := sem.Acquire(ctx, 1); err != nil {
				return metadataMsg{nonce: url, err: err}
			}
			defer sem.Release(1)

			meta, err := fetcher.Dispatch(ctx, url, client, fetchers)
			return metadataMsg{nonce: url, meta: meta, err: err}
		},
	)
}

// prefetchNext schedules metadata fetches for the next 3 non-directory entries.
func (m *Model) prefetchNext() tea.Cmd {
	var cmds []tea.Cmd
	count := 0
	cacheInst := m.cache
	for i := m.cursor + 1; i < len(m.entries) && count < 3; i++ {
		e := m.entries[i]
		if e.IsDir || m.prefetched[e.URL] {
			continue
		}
		if m.cache != nil {
			if _, _, err := m.cache.GetMetadata(e.URL); err == nil {
				continue // already cached
			}
		}
		m.prefetched[e.URL] = true
		eURL := e.URL
		client := m.client
		fetchers := m.fetchers
		sem := m.sem
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := sem.Acquire(ctx, 1); err != nil {
				return nil
			}
			defer sem.Release(1)
			meta, err := fetcher.Dispatch(ctx, eURL, client, fetchers)
			if err == nil && meta != nil && cacheInst != nil {
				_ = cacheInst.SetMetadata(eURL, meta, "")
			}
			return nil // discard — just a background warm-up
		})
		count++
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// spinnerCmd sends a spinner tick after 80ms.
func spinnerCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(80 * time.Millisecond)
		return spinnerTickMsg{}
	}
}

// View renders the two-pane TUI layout.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	totalHeight := m.height - 2 // reserve rows for status + help
	if totalHeight < 4 {
		totalHeight = 4
	}

	// Split width: 55% list / 45% meta, with border overhead.
	borderOverhead := 4 // 2 borders × 2 sides (left+right)
	available := m.width - borderOverhead
	listW := available * 55 / 100
	metaW := available - listW

	listContent := m.renderList(listW, totalHeight-2)
	metaContent := m.renderMeta(metaW, totalHeight-2)

	leftPane := listPaneStyle.
		Width(listW).
		Height(totalHeight - 2).
		Render(listContent)

	rightPane := metaPaneStyle.
		Width(metaW).
		Height(totalHeight - 2).
		Render(metaContent)

	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	status := m.renderStatus()
	help := helpStyle.Width(m.width).Render(
		"↑/k up  ↓/j down  l/enter dir  h/backspace back  s bytes  r retry  q quit",
	)

	return lipgloss.JoinVertical(lipgloss.Left, panes, status, help)
}

func (m Model) renderList(width, height int) string {
	if m.loadingListing {
		spin := spinnerStyle.Render(spinnerFrames[m.spinFrame])
		return spin + " Loading…"
	}
	if m.listingErr != nil {
		return metaErrStyle.Render("Error: " + m.listingErr.Error())
	}
	if len(m.entries) == 0 {
		return normalStyle.Render("(empty directory)")
	}

	var lines []string
	for i, e := range m.entries {
		name := e.Name
		suffix := ""
		if e.IsDir {
			suffix = "/"
			if e.DirSize != nil {
				suffix += fmt.Sprintf("  %s", formatDirSize(e.DirSize, m.showBytes))
			}
		} else if e.Size >= 0 {
			suffix = "  " + formatSize(e.Size, m.showBytes)
		}
		line := formatName(name, e.IsDir) + suffix

		var style lipgloss.Style
		if i == m.cursor {
			style = cursorStyle.Width(width)
		} else if e.IsDir {
			style = dirStyle
		} else {
			style = normalStyle
		}
		lines = append(lines, style.Render(truncate(line, width)))
	}

	// Window the visible lines around the cursor.
	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func (m Model) renderMeta(width, _ int) string {
	if len(m.entries) == 0 {
		return ""
	}
	if m.cursor >= len(m.entries) {
		return ""
	}
	e := m.entries[m.cursor]
	if e.IsDir {
		var sb strings.Builder
		sb.WriteString(metaTitleStyle.Render(e.Name + "/"))
		sb.WriteString("\n\n")
		if e.DirSize != nil {
			sb.WriteString(row("Files", fmt.Sprintf("%d", e.DirSize.FileCount)))
			sb.WriteString(row("Size", formatSize(e.DirSize.TotalSize, m.showBytes)))
			if e.DirSize.Partial {
				sb.WriteString(metaErrStyle.Render("(partial — subdirs not yet cached)"))
			}
		} else {
			sb.WriteString(normalStyle.Render("(size unknown — browse to cache)"))
		}
		return sb.String()
	}

	var sb strings.Builder
	sb.WriteString(metaTitleStyle.Render(truncate(e.Name, width-2)))
	sb.WriteString("\n")

	if e.Size >= 0 {
		sb.WriteString(row("Size", formatSize(e.Size, m.showBytes)))
	}
	if !e.ModTime.IsZero() {
		sb.WriteString(row("Modified", e.ModTime.Format("2006-01-02 15:04")))
	}

	sb.WriteString("\n")

	if m.fetching {
		spin := spinnerStyle.Render(spinnerFrames[m.spinFrame])
		sb.WriteString(spin + " Fetching metadata…")
		return sb.String()
	}

	if m.metaErr != nil {
		sb.WriteString(metaErrStyle.Render(metaErrText(m.metaErr)))
		return sb.String()
	}

	if m.metadata == nil {
		if m.fetchNonce == e.URL {
			sb.WriteString(normalStyle.Render("(waiting…)"))
		}
		return sb.String()
	}

	meta := m.metadata
	if meta.Format != "" {
		sb.WriteString(row("Format", meta.Format))
	}
	if meta.Duration > 0 {
		sb.WriteString(row("Duration", formatDuration(meta.Duration)))
	}
	if meta.Codec != "" {
		sb.WriteString(row("Codec", meta.Codec))
	}
	if meta.AudioInfo != "" {
		sb.WriteString(row("Audio", meta.AudioInfo))
	}
	if meta.RangeFetched > 0 {
		sb.WriteString(row("Fetched", formatSize(meta.RangeFetched, m.showBytes)+" via ranges"))
	}
	return sb.String()
}

func (m Model) renderStatus() string {
	if m.listingErr != nil {
		return statusErrStyle.Width(m.width).Render(
			"Error fetching listing — press r to retry",
		)
	}
	path := m.baseURL
	count := fmt.Sprintf("%d items", len(m.entries))
	histLen := fmt.Sprintf("depth %d", len(m.history))
	return statusBarStyle.Width(m.width).Render(
		fmt.Sprintf("%s  [%s, %s]", truncate(path, m.width-30), count, histLen),
	)
}

// --- Helpers ---

func row(label, value string) string {
	return metaLabelStyle.Render(label+":") + " " + metaValueStyle.Render(value) + "\n"
}

func formatName(name string, isDir bool) string {
	if isDir {
		return name + "/"
	}
	return name
}

func formatSize(n int64, showBytes bool) string {
	if showBytes {
		return fmt.Sprintf("%d B", n)
	}
	return humanize.Bytes(uint64(n))
}

func formatDirSize(d *cache.DirSize, showBytes bool) string {
	s := fmt.Sprintf("%d files / %s", d.FileCount, formatSize(d.TotalSize, showBytes))
	if d.Partial {
		s += " (partial)"
	}
	return s
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func metaErrText(err error) string {
	switch {
	case errors.Is(err, fetcher.ErrRangeUnsupported):
		return "Range requests not supported by server"
	case errors.Is(err, fetcher.ErrRangeIgnored):
		return "Server ignored Range header (returned 200)"
	case errors.Is(err, fetcher.ErrNoContentLength):
		return "Content-Length missing — cannot seek"
	case errors.Is(err, fetcher.ErrNoMatch):
		return "Metadata unavailable for this format"
	default:
		return "Error: " + err.Error()
	}
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

// fetchListingCmd issues a directory listing fetch as a tea.Cmd.
func fetchListingCmd(url string, c cache.Cache, client *http.Client, lister parser.DirectoryLister) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		entries, err := lister.List(ctx, url, client)
		return listingMsg{url: url, entries: entries, err: err}
	}
}
