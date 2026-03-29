package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubMetaFetcher is a test double for MetadataFetcher.
type stubMetaFetcher struct {
	ext  string
	meta *Metadata
	err  error
}

func (s stubMetaFetcher) Supports(e string) bool { return e == s.ext }
func (s stubMetaFetcher) Fetch(_ context.Context, _ string, _ *http.Client) (*Metadata, error) {
	return s.meta, s.err
}

// Regression: ISSUE-001 — Dispatch returns ErrRangeUnsupported when Accept-Ranges is "none"
// Found by /qa on 2026-03-29
// Report: .gstack/qa-reports/qa-report-fspeek-2026-03-29.md
func TestDispatch_AcceptRangesNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "none")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := Dispatch(context.Background(), srv.URL+"/file.mp4", srv.Client(), nil)
	if !errors.Is(err, ErrRangeUnsupported) {
		t.Errorf("want ErrRangeUnsupported, got %v", err)
	}
}

// Regression: ISSUE-001 — Dispatch returns ErrRangeUnsupported when Accept-Ranges header absent
// Found by /qa on 2026-03-29
// Report: .gstack/qa-reports/qa-report-fspeek-2026-03-29.md
func TestDispatch_AcceptRangesAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// no Accept-Ranges header
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := Dispatch(context.Background(), srv.URL+"/file.mp4", srv.Client(), nil)
	if !errors.Is(err, ErrRangeUnsupported) {
		t.Errorf("want ErrRangeUnsupported, got %v", err)
	}
}

// Regression: ISSUE-001 — Dispatch returns ErrNoMatch when no fetcher supports the extension
// Found by /qa on 2026-03-29
// Report: .gstack/qa-reports/qa-report-fspeek-2026-03-29.md
func TestDispatch_NoMatchingFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := Dispatch(context.Background(), srv.URL+"/file.mp3", srv.Client(), []MetadataFetcher{
		stubMetaFetcher{ext: "mp4"},
	})
	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("want ErrNoMatch, got %v", err)
	}
}

// Regression: ISSUE-001 — Dispatch delegates to matching fetcher and returns its result
// Found by /qa on 2026-03-29
// Report: .gstack/qa-reports/qa-report-fspeek-2026-03-29.md
func TestDispatch_MatchingFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	want := &Metadata{Format: "mp4"}
	got, err := Dispatch(context.Background(), srv.URL+"/file.mp4", srv.Client(), []MetadataFetcher{
		stubMetaFetcher{ext: "mp4", meta: want},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
