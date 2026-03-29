// Package cache provides a persistent cache for directory listings and file
// metadata, backed by SQLite with WAL mode for concurrent access.
package cache

import (
	"net/url"
	"time"

	"github.com/steadyfall/fspeek/fetcher"
)

// DirSize holds aggregated size information for a cached directory.
type DirSize struct {
	FileCount int64
	TotalSize int64
	// Partial is true when subdirectories have not been fully cached yet.
	Partial bool
}

// Entry represents a single item in a directory listing.
type Entry struct {
	Name    string
	URL     string
	IsDir   bool
	Size    int64 // -1 if unknown
	DirSize *DirSize
	ModTime time.Time
}

// Cache is the interface for persisting directory listings and file metadata.
type Cache interface {
	// GetListing returns cached entries for the given URL.
	// Returns the stored ETag and ErrCacheMiss if not found or expired.
	GetListing(rawURL string) ([]Entry, string, error)
	// SetListing stores directory entries with an optional ETag.
	SetListing(rawURL string, entries []Entry, etag string) error
	// GetMetadata returns cached metadata for the given file URL.
	GetMetadata(rawURL string) (*fetcher.Metadata, string, error)
	// SetMetadata stores file metadata with an optional ETag.
	SetMetadata(rawURL string, m *fetcher.Metadata, etag string) error
	// Invalidate removes all cached data for the given URL.
	Invalidate(rawURL string) error
	// ComputeDirSize recursively sums file sizes from the cached listings.
	// Returns nil if the directory has no cached listing.
	ComputeDirSize(rawURL string) *DirSize
	// Close releases resources held by the cache.
	Close() error
}

// Canonicalize normalizes a URL for use as a cache key:
//   - Percent-encoding is normalized via the standard url package round-trip.
//   - Query parameters are stripped.
//   - Fragment is stripped.
func Canonicalize(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// Strip query and fragment.
	u.RawQuery = ""
	u.Fragment = ""
	// Clear RawPath so url.String() re-encodes Path using standard encoding,
	// avoiding double-encoding of already-decoded paths.
	u.RawPath = ""
	return u.String()
}
