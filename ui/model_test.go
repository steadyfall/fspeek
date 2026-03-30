package ui

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/steadyfall/fspeek/cache"
	"github.com/steadyfall/fspeek/fetcher"
)

// --- Stubs ---

type stubCache struct {
	listings map[string][]cache.Entry
	metadata map[string]*fetcher.Metadata
}

func newStubCache() *stubCache {
	return &stubCache{
		listings: map[string][]cache.Entry{},
		metadata: map[string]*fetcher.Metadata{},
	}
}
func (s *stubCache) GetListing(url string) ([]cache.Entry, string, error) {
	if e, ok := s.listings[url]; ok {
		return e, "", nil
	}
	return nil, "", cache.ErrCacheMiss
}
func (s *stubCache) SetListing(url string, entries []cache.Entry, etag string) error {
	s.listings[url] = entries
	return nil
}
func (s *stubCache) GetMetadata(url string) (*fetcher.Metadata, string, error) {
	if m, ok := s.metadata[url]; ok {
		return m, "", nil
	}
	return nil, "", cache.ErrCacheMiss
}
func (s *stubCache) SetMetadata(url string, m *fetcher.Metadata, etag string) error {
	s.metadata[url] = m
	return nil
}
func (s *stubCache) Invalidate(url string) error { return nil }
func (s *stubCache) ComputeDirSize(url string) *cache.DirSize {
	entries, ok := s.listings[url]
	if !ok {
		return nil
	}
	var total int64
	for _, e := range entries {
		if !e.IsDir && e.Size >= 0 {
			total += e.Size
		}
	}
	return &cache.DirSize{FileCount: int64(len(entries)), TotalSize: total}
}
func (s *stubCache) Close() error { return nil }

type stubLister struct {
	entries []cache.Entry
	err     error
}

func (l stubLister) List(_ context.Context, _ string, _ *http.Client) ([]cache.Entry, error) {
	return l.entries, l.err
}

// --- Tests ---

func TestNew_DefaultsAndInit(t *testing.T) {
	entries := []cache.Entry{
		{Name: "file.mp4", URL: "http://x/file.mp4", Size: 1024},
	}
	sc := newStubCache()
	m := New("http://x/", Options{
		Cache:  sc,
		Client: http.DefaultClient,
		Lister: stubLister{entries: entries},
	})

	if m.baseURL != "http://x/" {
		t.Errorf("baseURL = %q, want %q", m.baseURL, "http://x/")
	}
	if m.sem == nil {
		t.Error("sem is nil")
	}
}

func TestUpdate_ListingMsg_SetsEntries(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{
		Cache:  sc,
		Client: http.DefaultClient,
		Lister: stubLister{},
	})

	entries := []cache.Entry{
		{Name: "a.mp4", URL: "http://x/a.mp4", IsDir: false, Size: 512},
		{Name: "sub/", URL: "http://x/sub/", IsDir: true, Size: -1},
	}
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: entries})
	m2 := newM.(Model)

	if len(m2.entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(m2.entries))
	}
	if m2.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m2.cursor)
	}
	if m2.listingErr != nil {
		t.Errorf("listingErr = %v, want nil", m2.listingErr)
	}
}

func TestUpdate_ListingMsg_StaleMsgIgnored(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.baseURL = "http://x/newdir/"

	entries := []cache.Entry{{Name: "file.mp4"}}
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: entries})
	m2 := newM.(Model)

	if len(m2.entries) != 0 {
		t.Error("stale listingMsg should not update entries")
	}
}

func TestUpdate_ListingMsg_Error(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	newM, _ := m.Update(listingMsg{url: "http://x/", err: errors.New("connection refused")})
	m2 := newM.(Model)

	if m2.listingErr == nil {
		t.Error("listingErr should be set on error")
	}
}

func TestUpdate_MetadataMsg_SetsMetadata(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.entries = []cache.Entry{{Name: "a.mp4", URL: "http://x/a.mp4"}}
	m.fetchNonce = "http://x/a.mp4"
	m.fetching = true

	meta := &fetcher.Metadata{Format: "Video / MP4", Duration: 90 * time.Second}
	newM, _ := m.Update(metadataMsg{nonce: "http://x/a.mp4", meta: meta})
	m2 := newM.(Model)

	if m2.metadata == nil {
		t.Fatal("metadata is nil")
	}
	if m2.metadata.Format != "Video / MP4" {
		t.Errorf("Format = %q", m2.metadata.Format)
	}
	if m2.fetching {
		t.Error("fetching should be false after result")
	}
}

func TestUpdate_MetadataMsg_Stale(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.fetchNonce = "http://x/current.mp4"

	newM, _ := m.Update(metadataMsg{nonce: "http://x/old.mp4", meta: &fetcher.Metadata{}})
	m2 := newM.(Model)

	if m2.metadata != nil {
		t.Error("stale metadataMsg should not update metadata")
	}
}

func TestUpdate_KeyQ_Quits(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	// Execute the cmd and check it's tea.Quit.
	msg := cmd()
	if msg != tea.Quit() {
		t.Errorf("expected tea.Quit msg, got %T", msg)
	}
}

func TestUpdate_KeyEsc_QuitsInNormalMode(t *testing.T) {
	// Regression: ISSUE-001 — esc in normal mode was a no-op; help bar advertised "esc exit"
	// Found by /qa on 2026-03-31
	// Report: .gstack/qa-reports/qa-report-directory-pane-customization-2026-03-31.md
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc in normal mode: expected quit cmd, got nil")
	}
	msg := cmd()
	if msg != tea.Quit() {
		t.Errorf("esc in normal mode: expected tea.Quit, got %T", msg)
	}
}

func TestUpdate_KeyEsc_ClearsFilterInFilterMode(t *testing.T) {
	// Regression: esc in filter mode should clear filter, NOT quit
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true
	m.filterQuery = "foo"
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m2 := newM.(Model)
	if cmd != nil {
		t.Errorf("esc in filter mode: expected nil cmd (not quit), got non-nil")
	}
	if m2.filterMode {
		t.Error("esc in filter mode: filterMode should be false")
	}
	if m2.filterQuery != "" {
		t.Errorf("esc in filter mode: filterQuery should be empty, got %q", m2.filterQuery)
	}
}

func TestUpdate_CursorMovement(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.entries = []cache.Entry{
		{Name: "a.mp4", URL: "http://x/a.mp4"},
		{Name: "b.mkv", URL: "http://x/b.mkv"},
		{Name: "c.srt", URL: "http://x/c.srt"},
	}
	m.cursor = 0

	// Move down.
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if newM.(Model).cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", newM.(Model).cursor)
	}

	// Move up.
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if newM.(Model).cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", newM.(Model).cursor)
	}

	// Can't go above 0.
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if newM.(Model).cursor != 0 {
		t.Errorf("cursor after k at top = %d, want 0", newM.(Model).cursor)
	}
}

func TestUpdate_ToggleBytes(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	if m.showBytes {
		t.Error("showBytes should default to false")
	}
	// 'b' now toggles bytes (was 's' before key rebind).
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if !newM.(Model).showBytes {
		t.Error("showBytes should be true after b")
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello w…"},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"", 5, ""},
	}
	for _, c := range cases {
		got := truncate(c.s, c.max)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{90 * time.Second, "1:30"},
		{2*time.Hour + 3*time.Minute + 4*time.Second, "2:03:04"},
		{5 * time.Minute, "5:00"},
	}
	for _, c := range cases {
		got := formatDuration(c.d)
		if got != c.want {
			t.Errorf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestMetaErrText(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fetcher.ErrRangeUnsupported, "Range requests not supported by server"},
		{fetcher.ErrRangeIgnored, "Server ignored Range header (returned 200)"},
		{fetcher.ErrNoContentLength, "Content-Length missing — cannot seek"},
		{fetcher.ErrNoMatch, "Metadata unavailable for this format"},
		{errors.New("boom"), "Error: boom"},
	}
	for _, c := range cases {
		got := metaErrText(c.err)
		if got != c.want {
			t.Errorf("metaErrText(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestView_BasicRender(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.width = 120
	m.height = 30
	m.entries = []cache.Entry{
		{Name: "movie.mp4", URL: "http://x/movie.mp4", Size: 1024 * 1024},
		{Name: "subs/", URL: "http://x/subs/", IsDir: true},
	}

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
	if !containsStr(view, "movie.mp4") {
		t.Error("View() does not contain 'movie.mp4'")
	}
}

// Regression: ISSUE-003 — formatDirSize renders count/size and optional (partial) suffix
// Found by /qa on 2026-03-29
// Report: .gstack/qa-reports/qa-report-fspeek-2026-03-29.md
func TestFormatDirSize(t *testing.T) {
	d := &cache.DirSize{FileCount: 3, TotalSize: 1024, Partial: false}
	got := formatDirSize(d, true)
	want := "3 files / 1024 B"
	if got != want {
		t.Errorf("formatDirSize(partial=false, bytes=true) = %q, want %q", got, want)
	}

	d.Partial = true
	got = formatDirSize(d, true)
	want = "3 files / 1024 B (partial)"
	if got != want {
		t.Errorf("formatDirSize(partial=true, bytes=true) = %q, want %q", got, want)
	}
}

// Regression: ISSUE-004 — back navigation cycles between current dir and immediate
// parent instead of traversing the full history stack.
func TestUpdate_BackNav_DeepHistory(t *testing.T) {
	sc := newStubCache()
	sc.listings["http://x/"] = []cache.Entry{
		{Name: "a/", URL: "http://x/a/", IsDir: true},
	}
	sc.listings["http://x/a/"] = []cache.Entry{
		{Name: "b/", URL: "http://x/a/b/", IsDir: true},
	}
	sc.listings["http://x/a/b/"] = []cache.Entry{
		{Name: "file.mkv", URL: "http://x/a/b/file.mkv"},
	}

	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: sc.listings["http://x/"]})
	m = newM.(Model)

	// Forward: root -> a/
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = newM.(Model)
	// Forward: a/ -> a/b/
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = newM.(Model)

	if m.baseURL != "http://x/a/b/" {
		t.Fatalf("after 2 forward navs: baseURL=%q, want http://x/a/b/", m.baseURL)
	}
	if len(m.history) != 2 {
		t.Fatalf("after 2 forward navs: history len=%d, want 2: %v", len(m.history), m.history)
	}

	// Back once: must land at a/, not re-enter a/b/
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = newM.(Model)
	if m.baseURL != "http://x/a/" {
		t.Errorf("after 1st back: baseURL=%q, want http://x/a/", m.baseURL)
	}
	if len(m.history) != 1 {
		t.Errorf("after 1st back: history len=%d, want 1: %v", len(m.history), m.history)
	}

	// Back twice: must land at root
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = newM.(Model)
	if m.baseURL != "http://x/" {
		t.Errorf("after 2nd back: baseURL=%q, want http://x/", m.baseURL)
	}
	if len(m.history) != 0 {
		t.Errorf("after 2nd back: history len=%d, want 0: %v", len(m.history), m.history)
	}
}

// Regression: navigateTo must clear loadingListing and listingErr on a cache hit.
// Before the fix, navigating to a cached URL after an error left the error state
// visible in the UI even though fresh entries were already populated from cache.
func TestNavigateTo_CachedHit_ClearsLoadingState(t *testing.T) {
	sc := newStubCache()
	sc.listings["http://x/sub/"] = []cache.Entry{
		{Name: "file.mkv", URL: "http://x/sub/file.mkv"},
	}

	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})

	// Simulate a prior listing error and loading state being active.
	m.loadingListing = true
	m.listingErr = errors.New("previous error")

	// Navigate to a URL that is in the cache.
	newM, _ := m.navigateTo("http://x/sub/", true)
	m2 := newM.(Model)

	if m2.loadingListing {
		t.Error("loadingListing should be false after cache hit")
	}
	if m2.listingErr != nil {
		t.Errorf("listingErr should be nil after cache hit, got %v", m2.listingErr)
	}
	if len(m2.entries) != 1 {
		t.Errorf("entries len = %d, want 1", len(m2.entries))
	}
	if m2.baseURL != "http://x/sub/" {
		t.Errorf("baseURL = %q, want %q", m2.baseURL, "http://x/sub/")
	}
}

// Regression: formatName must return plain text with no embedded ANSI escape codes.
// Previously, dirStyle and normalStyle were pre-applied inside formatName, embedding
// color codes into the string before the row-level cursor style was applied. When the
// cursor row then wrapped the line in cursorStyle, the embedded foreground color
// persisted and overrode the cursor's foreground — making directory names unreadable
// (electric blue on cyan background).
func TestFormatName_ReturnsPlainText(t *testing.T) {
	got := formatName("subdir", true)
	if got != "subdir/" {
		t.Errorf("formatName(dir) = %q, want plain %q (no embedded ANSI codes)", got, "subdir/")
	}
	got = formatName("movie.mp4", false)
	if got != "movie.mp4" {
		t.Errorf("formatName(file) = %q, want plain %q (no embedded ANSI codes)", got, "movie.mp4")
	}
}

func TestClampCursor(t *testing.T) {
	cases := []struct {
		saved, max, want int
	}{
		{0, 0, 0},  // empty dir
		{3, 0, 0},  // empty dir, non-zero saved
		{0, 5, 0},  // first visit (map miss -> Go zero value)
		{1, 5, 1},  // normal
		{4, 5, 4},  // last valid index
		{5, 5, 4},  // saved == max -> clamp to max-1
		{9, 5, 4},  // saved > max
		{-1, 5, 0}, // negative (defensive)
	}
	for _, c := range cases {
		got := clampCursor(c.saved, c.max)
		if got != c.want {
			t.Errorf("clampCursor(%d, %d) = %d, want %d", c.saved, c.max, got, c.want)
		}
	}
}

// --- padRight tests ---

func TestPadRight_ASCII(t *testing.T) {
	got := padRight("hello", 10)
	if got != "hello     " {
		t.Errorf("padRight ASCII: got %q, want %q", got, "hello     ")
	}
	if len([]rune(got)) != 10 {
		t.Errorf("padRight ASCII: display width %d, want 10", len([]rune(got)))
	}
}

func TestPadRight_Unicode(t *testing.T) {
	// "日本" — two CJK chars, each display width 2 → total display width 4.
	got := padRight("日本", 8)
	// Should pad 4 spaces to reach display width 8.
	if got != "日本    " {
		t.Errorf("padRight CJK: got %q, want %q", got, "日本    ")
	}
}

func TestPadRight_AlreadyWide(t *testing.T) {
	got := padRight("hello world", 5)
	if got != "hello world" {
		t.Errorf("padRight wider than target: got %q, want unchanged", got)
	}
}

// --- visibleEntries tests ---

func TestVisibleEntries_NoFilter(t *testing.T) {
	m := Model{entries: []cache.Entry{
		{Name: "a.mp4"}, {Name: "b.mkv"},
	}}
	got := m.visibleEntries()
	if len(got) != 2 {
		t.Errorf("no filter: len=%d, want 2", len(got))
	}
}

func TestVisibleEntries_Filter(t *testing.T) {
	m := Model{
		entries: []cache.Entry{
			{Name: "movie.mp4"}, {Name: "sub/"}, {Name: "movie2.mkv"},
		},
		filterQuery: "movie",
	}
	got := m.visibleEntries()
	if len(got) != 2 {
		t.Errorf("filter 'movie': len=%d, want 2", len(got))
	}
}

func TestVisibleEntries_NilEntries(t *testing.T) {
	m := Model{}
	got := m.visibleEntries()
	if got != nil {
		t.Errorf("nil entries: got %v, want nil", got)
	}
}

// --- sortEntries tests ---

func TestSortEntries_ByName(t *testing.T) {
	entries := []cache.Entry{
		{Name: "c"}, {Name: "a"}, {Name: "b"},
	}
	sortEntries(entries, SortByName)
	if entries[0].Name != "a" || entries[1].Name != "b" || entries[2].Name != "c" {
		t.Errorf("SortByName: got %v", entries)
	}
}

func TestSortEntries_ByCount(t *testing.T) {
	entries := []cache.Entry{
		{Name: "big/", IsDir: true, DirSize: &cache.DirSize{FileCount: 10}},
		{Name: "small/", IsDir: true, DirSize: &cache.DirSize{FileCount: 2}},
		{Name: "mid/", IsDir: true, DirSize: &cache.DirSize{FileCount: 5}},
	}
	sortEntries(entries, SortByCount)
	if entries[0].DirSize.FileCount != 2 || entries[1].DirSize.FileCount != 5 || entries[2].DirSize.FileCount != 10 {
		t.Errorf("SortByCount: got %v", entries)
	}
}

func TestSortEntries_BySize(t *testing.T) {
	entries := []cache.Entry{
		{Name: "big.mp4", Size: 1000},
		{Name: "small.mp4", Size: 100},
		{Name: "mid.mp4", Size: 500},
	}
	sortEntries(entries, SortBySize)
	if entries[0].Size != 100 || entries[1].Size != 500 || entries[2].Size != 1000 {
		t.Errorf("SortBySize: got %v", entries)
	}
}

func TestSortEntries_NilDirSize(t *testing.T) {
	entries := []cache.Entry{
		{Name: "a/", IsDir: true, DirSize: &cache.DirSize{FileCount: 5}},
		{Name: "b/", IsDir: true, DirSize: nil}, // nil → sorts to bottom
	}
	sortEntries(entries, SortByCount)
	// nil DirSize → sentinel MaxInt64, sorts last in ascending.
	if entries[0].Name != "a/" {
		t.Errorf("SortByCount with nil DirSize: expected a/ first (nil last), got %s", entries[0].Name)
	}
}

func TestSortEntries_Nil(t *testing.T) {
	// Must not panic.
	sortEntries(nil, SortByName)
	sortEntries(nil, SortByCount)
	sortEntries(nil, SortBySize)
}

// --- handleKey sort/filter tests ---

func TestHandleKey_SortCycle(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.entries = []cache.Entry{{Name: "a.mp4"}}

	steps := []struct {
		want SortBy
		name string
	}{
		{SortByCount, "SortByCount"},
		{SortBySize, "SortBySize"},
		{SortByNameDesc, "SortByNameDesc"},
		{SortByCountDesc, "SortByCountDesc"},
		{SortBySizeDesc, "SortBySizeDesc"},
		{SortByName, "SortByName (wrap)"},
	}

	cur := tea.Model(m)
	for i, step := range steps {
		newM, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		cur = newM
		if newM.(Model).sortBy != step.want {
			t.Errorf("after %d s: sortBy=%d, want %s(%d)", i+1, newM.(Model).sortBy, step.name, step.want)
		}
	}
}

func TestHandleKey_BytesRebound(t *testing.T) {
	// Regression: 'b' must toggle showBytes ('s' was rebound to sort cycle).
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	if m.showBytes {
		t.Error("showBytes should default to false")
	}
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if !newM.(Model).showBytes {
		t.Error("showBytes should be true after b")
	}
	// 's' must NOT toggle showBytes.
	m.entries = []cache.Entry{{Name: "a.mp4"}}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if newM.(Model).showBytes {
		t.Error("showBytes should remain false after s (s cycles sort, not bytes)")
	}
}

func TestHandleKey_FilterEnter(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m2 := newM.(Model)
	if !m2.filterMode {
		t.Error("filterMode should be true after /")
	}
}

func TestHandleKey_FilterRune(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m2 := newM.(Model)
	if m2.filterQuery != "f" {
		t.Errorf("filterQuery after 'f': %q, want %q", m2.filterQuery, "f")
	}

	newM, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	m3 := newM.(Model)
	if m3.filterQuery != "fo" {
		t.Errorf("filterQuery after 'fo': %q, want %q", m3.filterQuery, "fo")
	}
}

func TestHandleKey_FilterBackspace(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true
	m.filterQuery = "foo"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 := newM.(Model)
	if m2.filterQuery != "fo" {
		t.Errorf("filterQuery after backspace: %q, want %q", m2.filterQuery, "fo")
	}
}

func TestHandleKey_FilterEsc(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true
	m.filterQuery = "test"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m2 := newM.(Model)
	if m2.filterMode {
		t.Error("filterMode should be false after Esc")
	}
	if m2.filterQuery != "" {
		t.Errorf("filterQuery should be empty after Esc, got %q", m2.filterQuery)
	}
}

func TestHandleKey_FilterNavigation(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true
	m.filterQuery = "movie"
	m.entries = []cache.Entry{
		{Name: "movie1.mp4", URL: "http://x/movie1.mp4"},
		{Name: "sub/", URL: "http://x/sub/", IsDir: true},
		{Name: "movie2.mp4", URL: "http://x/movie2.mp4"},
	}
	m.cursor = 0

	// Down should move within filtered results (movie1, movie2).
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := newM.(Model)
	if m2.cursor != 1 {
		t.Errorf("cursor after down in filter: %d, want 1", m2.cursor)
	}
	// Can't go further (only 2 visible).
	newM, _ = m2.Update(tea.KeyMsg{Type: tea.KeyDown})
	m3 := newM.(Model)
	if m3.cursor != 1 {
		t.Errorf("cursor at bottom of filter: %d, want 1", m3.cursor)
	}
}

func TestHandleKey_FilterEnterNav(t *testing.T) {
	sc := newStubCache()
	sc.listings["http://x/sub/"] = []cache.Entry{}
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true
	m.filterQuery = "sub"
	m.entries = []cache.Entry{
		{Name: "movie.mp4", URL: "http://x/movie.mp4"},
		{Name: "sub/", URL: "http://x/sub/", IsDir: true},
	}
	m.cursor = 0 // cursor 0 in filtered view = sub/ (only match)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(Model)
	if m2.baseURL != "http://x/sub/" {
		t.Errorf("enter in filter nav: baseURL=%q, want http://x/sub/", m2.baseURL)
	}
}

func TestNavigateTo_ClearsFilter(t *testing.T) {
	sc := newStubCache()
	sc.listings["http://x/sub/"] = []cache.Entry{}
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true
	m.filterQuery = "sub"

	newM, _ := m.navigateTo("http://x/sub/", true)
	m2 := newM.(Model)
	if m2.filterMode {
		t.Error("filterMode should be cleared after navigation")
	}
	if m2.filterQuery != "" {
		t.Errorf("filterQuery should be empty after navigation, got %q", m2.filterQuery)
	}
}

// --- renderList tests ---

func makeModel(entries []cache.Entry) Model {
	m := Model{
		width:   120,
		height:  30,
		entries: entries,
	}
	return m
}

func TestRenderListColumnar_Dir(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024 * 1024},
		},
	})
	out := m.renderList(120, 20)
	if !containsStr(out, "folder1/") {
		t.Error("renderList columnar: missing dir name")
	}
	if !containsStr(out, "4 files") {
		t.Error("renderList columnar: missing file count")
	}
	if !containsStr(out, "NAME") {
		t.Error("renderList columnar: missing header NAME")
	}
	if !containsStr(out, "COUNT") {
		t.Error("renderList columnar: missing header COUNT")
	}
	if !containsStr(out, "SIZE") {
		t.Error("renderList columnar: missing header SIZE")
	}
}

func TestRenderListColumnar_Partial(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024, Partial: true},
		},
	})
	out := m.renderList(120, 20)
	if !containsStr(out, "~") {
		t.Error("renderList partial: missing ~ indicator")
	}
}

func TestRenderListColumnar_NoDirSize(t *testing.T) {
	m := makeModel([]cache.Entry{
		{Name: "folder1", IsDir: true, URL: "http://x/folder1/", DirSize: nil},
	})
	out := m.renderList(120, 20)
	if !containsStr(out, "folder1/") {
		t.Error("renderList no DirSize: missing dir name")
	}
}

func TestRenderListColumnar_File(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:  "movie.mp4",
			IsDir: false,
			URL:   "http://x/movie.mp4",
			Size:  1024 * 1024 * 1024,
		},
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 500 * 1024 * 1024},
		},
	})
	out := m.renderList(120, 20)
	if !containsStr(out, "movie.mp4") {
		t.Error("renderList file: missing filename")
	}
}

func TestRenderListColumnar_CursorRow(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 2, TotalSize: 1024},
		},
	})
	m.cursor = 0
	out := m.renderList(120, 20)
	// Cursor row is rendered by cursorStyle (cyan background) — just verify it renders.
	if !containsStr(out, "folder1/") {
		t.Error("renderList cursor row: missing name")
	}
}

func TestRenderListAdaptive_Narrow(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024 * 1024},
		},
	})
	// Very narrow terminal — falls back to compact format.
	out := m.renderList(20, 20)
	if !containsStr(out, "folder1/") {
		t.Error("renderList narrow: missing name")
	}
}

func TestRenderListAdaptive_NoStats(t *testing.T) {
	m := makeModel([]cache.Entry{
		{Name: "folder1", IsDir: true, URL: "http://x/folder1/", DirSize: nil},
		{Name: "folder2", IsDir: true, URL: "http://x/folder2/", DirSize: nil},
	})
	out := m.renderList(120, 20)
	// No stats → no column header.
	if containsStr(out, "COUNT") {
		t.Error("renderList no stats: should not show column header")
	}
}

func TestRenderListHeader_SortByName(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024},
		},
	})
	m.sortBy = SortByName
	out := m.renderList(120, 20)
	if !containsStr(out, "NAME") {
		t.Error("header SortByName: missing NAME")
	}
	if !containsStr(out, "COUNT") {
		t.Error("header SortByName: missing COUNT")
	}
	if !containsStr(out, "SIZE") {
		t.Error("header SortByName: missing SIZE")
	}
}

func TestRenderListHeader_SortBySize(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024},
		},
	})
	m.sortBy = SortBySize
	out := m.renderList(120, 20)
	if !containsStr(out, "▲") {
		t.Error("header SortBySize: missing ▲ indicator")
	}
}

func TestRenderListFilter_Match(t *testing.T) {
	m := makeModel([]cache.Entry{
		{Name: "movie.mp4", URL: "http://x/movie.mp4", Size: 1024},
		{Name: "sub/", URL: "http://x/sub/", IsDir: true, DirSize: &cache.DirSize{FileCount: 1}},
		{Name: "clip.mp4", URL: "http://x/clip.mp4", Size: 512},
	})
	m.filterQuery = "movie"
	out := m.renderList(120, 20)
	if !containsStr(out, "movie.mp4") {
		t.Error("filter match: should show movie.mp4")
	}
	if containsStr(out, "clip.mp4") {
		t.Error("filter match: should not show clip.mp4")
	}
}

func TestRenderListFilter_NoMatch(t *testing.T) {
	m := makeModel([]cache.Entry{
		{Name: "movie.mp4", URL: "http://x/movie.mp4", Size: 1024},
	})
	m.filterQuery = "zzz"
	out := m.renderList(120, 20)
	if !containsStr(out, "(no matches)") {
		t.Error("filter no match: should show (no matches)")
	}
}

func TestRenderListFilter_CaseInsensitive(t *testing.T) {
	m := makeModel([]cache.Entry{
		{Name: "Foobar.mp4", URL: "http://x/Foobar.mp4", Size: 1024},
		{Name: "other.mkv", URL: "http://x/other.mkv", Size: 512},
	})
	m.filterQuery = "foo"
	out := m.renderList(120, 20)
	if !containsStr(out, "Foobar.mp4") {
		t.Error("filter case-insensitive: should match Foobar.mp4 with query 'foo'")
	}
	if containsStr(out, "other.mkv") {
		t.Error("filter case-insensitive: should not show other.mkv")
	}
}

// --- renderStatus filter tests ---

func TestRenderStatus_FilterMode(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.filterMode = true
	m.filterQuery = "test"
	m.width = 80

	out := m.renderStatus()
	if !containsStr(out, "/ test_") {
		t.Errorf("renderStatus filter mode: got %q, want to contain '/ test_'", out)
	}
}

// stripANSI removes ANSI SGR escape sequences from s for plain-text comparison.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// --- Files-only columnar alignment test ---

func TestRenderListColumnar_FilesOnly(t *testing.T) {
	// Directory with only files (no subdirs) — maxCountW==0.
	// SIZE column in the header and data rows must start at the same offset.
	// Use SortByCount so neither NAME nor SIZE shows an indicator, keeping
	// the header prefix width equal to the data prefix width.
	m := makeModel([]cache.Entry{
		{Name: "movie.mp4", IsDir: false, URL: "http://x/movie.mp4", Size: 1024 * 1024 * 100},
		{Name: "clip.mp4", IsDir: false, URL: "http://x/clip.mp4", Size: 1024 * 512},
	})
	m.sortBy = SortByCount

	out := m.renderList(120, 20)

	rawLines := strings.Split(out, "\n")
	if len(rawLines) < 2 {
		t.Fatalf("FilesOnly: expected at least header + 1 data line, got %d lines", len(rawLines))
	}

	// Strip ANSI before measuring byte offsets.
	headerLine := stripANSI(rawLines[0])
	dataLine := stripANSI(rawLines[1])

	headerIdx := strings.Index(headerLine, "SIZE")
	if headerIdx < 0 {
		t.Fatalf("FilesOnly: SIZE not found in header %q", headerLine)
	}

	// Find size string for first entry — try humanize format variants.
	dataIdx := -1
	for _, candidate := range []string{"100 MB", "105 MB", "104 MB", "95 MB"} {
		if i := strings.Index(dataLine, candidate); i >= 0 {
			dataIdx = i
			break
		}
	}
	if dataIdx < 0 {
		t.Fatalf("FilesOnly: size string not found in data line %q", dataLine)
	}

	if headerIdx != dataIdx {
		t.Errorf("FilesOnly: SIZE header at col %d but size data at col %d (misaligned by %d)\n  header: %q\n  data:   %q",
			headerIdx, dataIdx, dataIdx-headerIdx, headerLine, dataLine)
	}
}

// --- Descending sort tests ---

func TestSortEntries_ByNameDesc(t *testing.T) {
	entries := []cache.Entry{
		{Name: "c"}, {Name: "a"}, {Name: "b"},
	}
	sortEntries(entries, SortByNameDesc)
	if entries[0].Name != "c" || entries[1].Name != "b" || entries[2].Name != "a" {
		t.Errorf("SortByNameDesc: got %v", entries)
	}
}

func TestSortEntries_ByCountDesc(t *testing.T) {
	entries := []cache.Entry{
		{Name: "big/", IsDir: true, DirSize: &cache.DirSize{FileCount: 10}},
		{Name: "small/", IsDir: true, DirSize: &cache.DirSize{FileCount: 2}},
		{Name: "mid/", IsDir: true, DirSize: &cache.DirSize{FileCount: 5}},
	}
	sortEntries(entries, SortByCountDesc)
	if entries[0].DirSize.FileCount != 10 || entries[1].DirSize.FileCount != 5 || entries[2].DirSize.FileCount != 2 {
		t.Errorf("SortByCountDesc: got %v", entries)
	}
}

func TestSortEntries_BySizeDesc(t *testing.T) {
	entries := []cache.Entry{
		{Name: "big.mp4", Size: 1000},
		{Name: "small.mp4", Size: 100},
		{Name: "mid.mp4", Size: 500},
	}
	sortEntries(entries, SortBySizeDesc)
	if entries[0].Size != 1000 || entries[1].Size != 500 || entries[2].Size != 100 {
		t.Errorf("SortBySizeDesc: got %v", entries)
	}
}

func TestSortEntries_NilDirSizeAsc(t *testing.T) {
	entries := []cache.Entry{
		{Name: "a/", IsDir: true, DirSize: nil},
		{Name: "b/", IsDir: true, DirSize: &cache.DirSize{FileCount: 3}},
		{Name: "c/", IsDir: true, DirSize: &cache.DirSize{FileCount: 1}},
	}
	sortEntries(entries, SortByCount)
	// nil DirSize → MaxInt64 sentinel → sorts last
	if entries[2].Name != "a/" {
		t.Errorf("SortByCount nil asc: nil should be last, got last=%s", entries[2].Name)
	}
}

func TestSortEntries_NilDirSizeDesc(t *testing.T) {
	entries := []cache.Entry{
		{Name: "a/", IsDir: true, DirSize: nil},
		{Name: "b/", IsDir: true, DirSize: &cache.DirSize{FileCount: 3}},
		{Name: "c/", IsDir: true, DirSize: &cache.DirSize{FileCount: 1}},
	}
	sortEntries(entries, SortByCountDesc)
	// nil DirSize → -1 sentinel → sorts last in descending
	if entries[2].Name != "a/" {
		t.Errorf("SortByCountDesc nil desc: nil should be last, got last=%s", entries[2].Name)
	}
}

// --- Header desc indicator tests ---

func TestRenderListHeader_SortByNameDesc(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024},
		},
	})
	m.sortBy = SortByNameDesc
	out := m.renderList(120, 20)
	if !containsStr(out, "▼") {
		t.Error("header SortByNameDesc: missing ▼ indicator")
	}
}

func TestRenderListHeader_SortByCountDesc(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024},
		},
	})
	m.sortBy = SortByCountDesc
	out := m.renderList(120, 20)
	if !containsStr(out, "▼") {
		t.Error("header SortByCountDesc: missing ▼ indicator")
	}
}

func TestRenderListHeader_SortBySizeDesc(t *testing.T) {
	m := makeModel([]cache.Entry{
		{
			Name:    "folder1",
			IsDir:   true,
			URL:     "http://x/folder1/",
			DirSize: &cache.DirSize{FileCount: 4, TotalSize: 1024 * 1024},
		},
	})
	m.sortBy = SortBySizeDesc
	out := m.renderList(120, 20)
	if !containsStr(out, "▼") {
		t.Error("header SortBySizeDesc: missing ▼ indicator")
	}
}

// --- Help text test ---

func TestView_HelpText(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.width = 200
	m.height = 30
	out := m.View()
	if !containsStr(out, "esc exit") {
		t.Error("help text: missing 'esc exit'")
	}
	if !containsStr(out, "backspace") {
		t.Error("help text: missing 'backspace'")
	}
	if !containsStr(out, "h/backspace/← back") {
		t.Error("help text: missing 'h/backspace/← back'")
	}
}

func TestPrefetchNext_RespectsFilter(t *testing.T) {
	// Regression: prefetchNext iterated m.entries with a cursor that indexes
	// visibleEntries(). With an active filter, it would queue invisible entries.
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.entries = []cache.Entry{
		{Name: "alpha.mp4", URL: "http://x/alpha.mp4", Size: 100},
		{Name: "beta.mp4", URL: "http://x/beta.mp4", Size: 200},
		{Name: "gamma.mp4", URL: "http://x/gamma.mp4", Size: 300},
		{Name: "delta.mp4", URL: "http://x/delta.mp4", Size: 400},
	}
	// Filter to only show delta — cursor=0 points to delta in visibleEntries,
	// but delta is at m.entries[3]. A buggy prefetchNext would start at m.entries[1].
	m.filterQuery = "delta"
	m.cursor = 0
	m.fetchNonce = "http://x/delta.mp4"

	newM, _ := m.Update(metadataMsg{
		nonce: "http://x/delta.mp4",
		meta:  &fetcher.Metadata{Format: "MP4"},
	})
	m2 := newM.(Model)

	// delta is the only visible entry at cursor=0; nothing comes after it.
	// alpha, beta, gamma must NOT be prefetched (not visible, not after cursor).
	for _, url := range []string{"http://x/alpha.mp4", "http://x/beta.mp4", "http://x/gamma.mp4"} {
		if m2.prefetched[url] {
			t.Errorf("prefetchNext with filter: %s should NOT be prefetched (not in filtered view)", url)
		}
	}
}

func TestCursorRestored_BackNav_CacheHit(t *testing.T) {
	sc := newStubCache()
	// root: two entries, subfolder at index 1
	root := []cache.Entry{
		{Name: "file.mp4", URL: "http://x/file.mp4"},
		{Name: "sub/", URL: "http://x/sub/", IsDir: true},
	}
	subEntries := []cache.Entry{
		{Name: "a.mkv", URL: "http://x/sub/a.mkv"},
	}
	sc.listings["http://x/"] = root
	sc.listings["http://x/sub/"] = subEntries

	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	// Load root.
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: root})
	m = newM.(Model)

	// Move cursor to index 1 (sub/).
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor before nav = %d, want 1", m.cursor)
	}

	// Navigate into sub/ (cache hit).
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = newM.(Model)
	if m.baseURL != "http://x/sub/" {
		t.Fatalf("baseURL = %q, want http://x/sub/", m.baseURL)
	}

	// Navigate back.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = newM.(Model)

	if m.baseURL != "http://x/" {
		t.Errorf("baseURL after back = %q, want http://x/", m.baseURL)
	}
	if m.cursor != 1 {
		t.Errorf("cursor after back = %d, want 1 (restored)", m.cursor)
	}
}

func TestCursorRestored_BackNav_CacheMiss(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	// Pre-populate cursorMap to simulate a previously-visited URL.
	m.cursorMap["http://x/"] = 3

	entries := []cache.Entry{
		{Name: "a.mp4", URL: "http://x/a.mp4"},
		{Name: "b.mkv", URL: "http://x/b.mkv"},
		{Name: "c.srt", URL: "http://x/c.srt"},
		{Name: "d.mp4", URL: "http://x/d.mp4"},
	}
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: entries})
	m2 := newM.(Model)

	if m2.cursor != 3 {
		t.Errorf("cursor after listingMsg = %d, want 3 (restored from cursorMap)", m2.cursor)
	}
}

func TestCursorClamped_DirectoryShrank(t *testing.T) {
	sc := newStubCache()
	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	m.cursorMap["http://x/"] = 5 // previously had cursor at 5

	entries := []cache.Entry{
		{Name: "a.mp4", URL: "http://x/a.mp4"},
		{Name: "b.mkv", URL: "http://x/b.mkv"},
		{Name: "c.srt", URL: "http://x/c.srt"},
	}
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: entries})
	m2 := newM.(Model)

	if m2.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped to len-1)", m2.cursor)
	}
}

func TestCursorRestored_E2E_CacheMissSave(t *testing.T) {
	sc := newStubCache()
	// root is in cache; sub is NOT (forces cache-miss path in navigateTo).
	root := []cache.Entry{
		{Name: "file.mp4", URL: "http://x/file.mp4"},
		{Name: "sub/", URL: "http://x/sub/", IsDir: true},
	}
	sc.listings["http://x/"] = root
	// sub/ intentionally NOT in cache.

	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: root})
	m = newM.(Model)

	// Move cursor to index 1 (sub/).
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor before nav = %d, want 1", m.cursor)
	}

	// Navigate into sub/ (cache miss - navigateTo fires save, then issues fetch).
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = newM.(Model)
	if m.baseURL != "http://x/sub/" {
		t.Fatalf("baseURL = %q, want http://x/sub/", m.baseURL)
	}
	// The save should have recorded cursor=1 for root.
	if m.cursorMap["http://x/"] != 1 {
		t.Errorf("cursorMap[root] = %d, want 1 (saved before cache-miss navigate)", m.cursorMap["http://x/"])
	}

	// Simulate listing arriving for sub/.
	subEntries := []cache.Entry{{Name: "a.mkv", URL: "http://x/sub/a.mkv"}}
	newM, _ = m.Update(listingMsg{url: "http://x/sub/", entries: subEntries})
	m = newM.(Model)

	// Navigate back to root (cache hit).
	sc.listings["http://x/"] = root // ensure root is still in cache
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = newM.(Model)

	if m.baseURL != "http://x/" {
		t.Errorf("baseURL = %q, want http://x/", m.baseURL)
	}
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (restored from cursorMap after cache-miss save)", m.cursor)
	}
}

// Regression: navigating to a cache-miss URL that later fails to load must not
// pollute cursorMap with the parent's cursor under the child's key.
func TestCursorNotCorrupted_FailedLoad(t *testing.T) {
	sc := newStubCache()
	root := []cache.Entry{
		{Name: "file.mp4", URL: "http://x/file.mp4"},
		{Name: "sub/", URL: "http://x/sub/", IsDir: true},
		{Name: "zzz.mkv", URL: "http://x/zzz.mkv"},
	}
	sc.listings["http://x/"] = root
	// sub/ NOT in cache.

	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: root})
	m = newM.(Model)

	// Move cursor to index 2.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)
	if m.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", m.cursor)
	}

	// Navigate into sub/ (cache miss) — cursor in root saved as 1, cursor reset to 0.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // back to index 1
	m = newM.(Model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // to index 2 again
	m = newM.(Model)
	// Navigate to sub/ from index 1 - move cursor to 1 first.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(Model) // cursor = 1 (on sub/)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = newM.(Model)
	if m.baseURL != "http://x/sub/" {
		t.Fatalf("baseURL = %q, want http://x/sub/", m.baseURL)
	}
	// cursor should be 0 (reset in cache-miss path).
	if m.cursor != 0 {
		t.Errorf("cursor after cache-miss nav = %d, want 0", m.cursor)
	}

	// Simulate fetch error for sub/.
	newM, _ = m.Update(listingMsg{url: "http://x/sub/", err: errors.New("connection refused")})
	m = newM.(Model)
	if m.listingErr == nil {
		t.Fatal("expected listingErr to be set")
	}

	// Navigate back to root.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = newM.(Model)
	if m.baseURL != "http://x/" {
		t.Fatalf("baseURL = %q, want http://x/", m.baseURL)
	}

	// Now navigate to sub/ again successfully.
	m.cursor = 1 // point at sub/
	subEntries := []cache.Entry{{Name: "a.mkv", URL: "http://x/sub/a.mkv"}}
	newM, _ = m.navigateTo("http://x/sub/", true)
	m = newM.(Model)
	newM, _ = m.Update(listingMsg{url: "http://x/sub/", entries: subEntries})
	m2 := newM.(Model)

	// Cursor should be 0 - the stale parent cursor (1) must NOT have been saved under sub/.
	if m2.cursor != 0 {
		t.Errorf("cursor in sub/ after successful revisit = %d, want 0 (must not restore stale parent cursor)", m2.cursor)
	}
}

// Regression: backing out of a cache-miss directory before its listing arrives
// must not overwrite a previously saved cursor for that directory with 0.
func TestCursorPreserved_BackoutDuringLoad(t *testing.T) {
	sc := newStubCache()
	root := []cache.Entry{
		{Name: "a.txt", URL: "http://x/a.txt"},
		{Name: "child/", URL: "http://x/child/", IsDir: true},
	}
	sc.listings["http://x/"] = root
	// child/ is NOT in cache — navigation will be a cache-miss.

	m := New("http://x/", Options{Cache: sc, Client: http.DefaultClient, Lister: stubLister{}})
	newM, _ := m.Update(listingMsg{url: "http://x/", entries: root})
	m = newM.(Model)

	// Simulate a prior visit to child/ with the cursor at position 1.
	m.cursorMap["http://x/child/"] = 1

	// Move root cursor onto child/ (index 1) and navigate in.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model) // cursor = 1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = newM.(Model)
	if m.baseURL != "http://x/child/" {
		t.Fatalf("baseURL = %q, want http://x/child/", m.baseURL)
	}
	if !m.loadingListing {
		t.Fatal("expected loadingListing=true after cache-miss navigation")
	}

	// Back out immediately — listing has NOT arrived yet.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = newM.(Model)
	if m.baseURL != "http://x/" {
		t.Fatalf("baseURL = %q, want http://x/", m.baseURL)
	}

	// The prior saved cursor for child/ must still be 1, not clobbered to 0.
	if got := m.cursorMap["http://x/child/"]; got != 1 {
		t.Errorf("cursorMap[child/] = %d, want 1 (must not be clobbered during in-flight load)", got)
	}
}

func containsStr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && (s == sub || len(s) >= len(sub) &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
