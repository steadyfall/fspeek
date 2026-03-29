package ui

import (
	"context"
	"errors"
	"net/http"
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
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !newM.(Model).showBytes {
		t.Error("showBytes should be true after s")
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
