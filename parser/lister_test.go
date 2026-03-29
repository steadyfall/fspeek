package parser

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/steadyfall/fspeek/cache"
)

type stubLister struct {
	entries []cache.Entry
	err     error
}

func (s stubLister) List(_ context.Context, _ string, _ *http.Client) ([]cache.Entry, error) {
	return s.entries, s.err
}

var someEntries = []cache.Entry{{Name: "file.mp4"}}

func TestCascade_FirstSucceeds(t *testing.T) {
	listers := []DirectoryLister{
		stubLister{entries: someEntries, err: nil},
		stubLister{err: errors.New("should not reach")},
	}
	got, err := Cascade(context.Background(), "http://x/", http.DefaultClient, listers)
	if err != nil {
		t.Fatalf("Cascade: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestCascade_SkipNoMatch(t *testing.T) {
	listers := []DirectoryLister{
		stubLister{err: ErrNoMatch},
		stubLister{entries: someEntries, err: nil},
	}
	got, err := Cascade(context.Background(), "http://x/", http.DefaultClient, listers)
	if err != nil {
		t.Fatalf("Cascade: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestCascade_AllNoMatch(t *testing.T) {
	listers := []DirectoryLister{
		stubLister{err: ErrNoMatch},
		stubLister{err: ErrNoMatch},
	}
	_, err := Cascade(context.Background(), "http://x/", http.DefaultClient, listers)
	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("want ErrNoMatch, got %v", err)
	}
}

func TestCascade_LastRealError(t *testing.T) {
	realErr := errors.New("connection refused")
	listers := []DirectoryLister{
		stubLister{err: ErrNoMatch},
		stubLister{err: realErr},
	}
	_, err := Cascade(context.Background(), "http://x/", http.DefaultClient, listers)
	if !errors.Is(err, realErr) {
		t.Errorf("want realErr, got %v", err)
	}
}

func TestCascade_EmptyResultIsSuccess(t *testing.T) {
	listers := []DirectoryLister{
		stubLister{entries: []cache.Entry{}, err: nil},
	}
	got, err := Cascade(context.Background(), "http://x/", http.DefaultClient, listers)
	if err != nil {
		t.Fatalf("Cascade: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

// Regression: ISSUE-002 — NewCascade returns a DirectoryLister that delegates to Cascade
// Found by /qa on 2026-03-29
// Report: .gstack/qa-reports/qa-report-fspeek-2026-03-29.md
func TestNewCascade_Delegates(t *testing.T) {
	lister := NewCascade(
		stubLister{err: ErrNoMatch},
		stubLister{entries: someEntries, err: nil},
	)
	got, err := lister.List(context.Background(), "http://x/", http.DefaultClient)
	if err != nil {
		t.Fatalf("NewCascade List: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

// Regression: ISSUE-002 — NewCascade List propagates ErrNoMatch when all listers fail
// Found by /qa on 2026-03-29
// Report: .gstack/qa-reports/qa-report-fspeek-2026-03-29.md
func TestNewCascade_AllNoMatch(t *testing.T) {
	lister := NewCascade(
		stubLister{err: ErrNoMatch},
		stubLister{err: ErrNoMatch},
	)
	_, err := lister.List(context.Background(), "http://x/", http.DefaultClient)
	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("want ErrNoMatch, got %v", err)
	}
}
