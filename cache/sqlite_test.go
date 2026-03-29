package cache

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/steadyfall/fspeek/fetcher"
)

func openTestCache(t *testing.T) *SQLiteCache {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	c, err := Open(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// --- Canonicalize ---

func TestCanonicalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			"https://example.com/media/",
			"https://example.com/media/",
		},
		{
			"https://example.com/media?foo=bar",
			"https://example.com/media",
		},
		{
			"https://example.com/media#frag",
			"https://example.com/media",
		},
		{
			"https://example.com/media%20files/",
			"https://example.com/media%20files/",
		},
	}
	for _, c := range cases {
		got := Canonicalize(c.in)
		if got != c.want {
			t.Errorf("Canonicalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- Listing CRUD ---

func TestSetGetListing(t *testing.T) {
	c := openTestCache(t)
	entries := []Entry{
		{Name: "file.mp4", URL: "https://ex.com/file.mp4", IsDir: false, Size: 1024},
		{Name: "subdir/", URL: "https://ex.com/subdir/", IsDir: true, Size: -1},
	}
	url := "https://ex.com/"

	if err := c.SetListing(url, entries, "etag-abc"); err != nil {
		t.Fatalf("SetListing: %v", err)
	}

	got, etag, err := c.GetListing(url)
	if err != nil {
		t.Fatalf("GetListing: %v", err)
	}
	if etag != "etag-abc" {
		t.Errorf("etag = %q, want %q", etag, "etag-abc")
	}
	if len(got) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(got))
	}
	if got[0].Name != "file.mp4" || got[1].IsDir != true {
		t.Errorf("unexpected entries: %+v", got)
	}
}

func TestGetListing_CacheMiss(t *testing.T) {
	c := openTestCache(t)
	_, _, err := c.GetListing("https://ex.com/missing/")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss, got %v", err)
	}
}

func TestGetListing_NoETagTTLExpiry(t *testing.T) {
	c := openTestCache(t)
	// Override TTL to 0 to simulate expiry.
	c.ttl = 0

	entries := []Entry{{Name: "f.mkv", URL: "https://ex.com/f.mkv"}}
	if err := c.SetListing("https://ex.com/", entries, ""); err != nil {
		t.Fatalf("SetListing: %v", err)
	}
	time.Sleep(time.Millisecond)

	_, _, err := c.GetListing("https://ex.com/")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss after TTL expiry, got %v", err)
	}
}

func TestInvalidate(t *testing.T) {
	c := openTestCache(t)
	url := "https://ex.com/"
	if err := c.SetListing(url, []Entry{{Name: "a.mp4"}}, "etag1"); err != nil {
		t.Fatalf("SetListing: %v", err)
	}
	if err := c.Invalidate(url); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	_, _, err := c.GetListing(url)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss after Invalidate, got %v", err)
	}
}

// --- Metadata CRUD ---

func TestSetGetMetadata(t *testing.T) {
	c := openTestCache(t)
	url := "https://ex.com/video.mp4"
	m := &fetcher.Metadata{
		Format:   "Video / MP4",
		Duration: 90 * time.Second,
		Codec:    "H.264/AVC",
	}
	if err := c.SetMetadata(url, m, "etag-xyz"); err != nil {
		t.Fatalf("SetMetadata: %v", err)
	}

	got, etag, err := c.GetMetadata(url)
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	if etag != "etag-xyz" {
		t.Errorf("etag = %q, want %q", etag, "etag-xyz")
	}
	if got.Format != m.Format || got.Duration != m.Duration || got.Codec != m.Codec {
		t.Errorf("unexpected metadata: %+v", got)
	}
}

func TestGetMetadata_CacheMiss(t *testing.T) {
	c := openTestCache(t)
	_, _, err := c.GetMetadata("https://ex.com/nosuchfile.mp4")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss, got %v", err)
	}
}

// --- ComputeDirSize ---

func TestComputeDirSize(t *testing.T) {
	c := openTestCache(t)

	// Root listing with two files and one subdir.
	c.SetListing("https://ex.com/", []Entry{
		{Name: "a.mp4", URL: "https://ex.com/a.mp4", IsDir: false, Size: 100},
		{Name: "b.mkv", URL: "https://ex.com/b.mkv", IsDir: false, Size: 200},
		{Name: "sub/", URL: "https://ex.com/sub/", IsDir: true, Size: -1},
	}, "")

	// Subdir listing.
	c.SetListing("https://ex.com/sub/", []Entry{
		{Name: "c.srt", URL: "https://ex.com/sub/c.srt", IsDir: false, Size: 50},
	}, "")

	ds := c.ComputeDirSize("https://ex.com/")
	if ds == nil {
		t.Fatal("ComputeDirSize returned nil")
	}
	if ds.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", ds.FileCount)
	}
	if ds.TotalSize != 350 {
		t.Errorf("TotalSize = %d, want 350", ds.TotalSize)
	}
	if ds.Partial {
		t.Error("Partial = true, want false")
	}
}

func TestComputeDirSize_Partial(t *testing.T) {
	c := openTestCache(t)

	// Root listing has subdir but subdir not cached.
	c.SetListing("https://ex.com/", []Entry{
		{Name: "a.mp4", URL: "https://ex.com/a.mp4", IsDir: false, Size: 100},
		{Name: "sub/", URL: "https://ex.com/sub/", IsDir: true, Size: -1},
	}, "")
	// No listing for sub/ — should mark Partial.

	ds := c.ComputeDirSize("https://ex.com/")
	if ds == nil {
		t.Fatal("ComputeDirSize returned nil")
	}
	if !ds.Partial {
		t.Error("Partial = false, want true")
	}
	if ds.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", ds.FileCount)
	}
}

func TestComputeDirSize_NoListing(t *testing.T) {
	c := openTestCache(t)
	ds := c.ComputeDirSize("https://ex.com/notcached/")
	if ds != nil {
		t.Errorf("expected nil for uncached dir, got %+v", ds)
	}
}
