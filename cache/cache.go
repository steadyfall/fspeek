// Package cache provides a persistent cache for directory listings and file
// metadata, backed by SQLite with WAL mode for concurrent access.
package cache

import (
	"net/url"
	"strings"
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
//   - Percent-encoding is normalized (decoded then re-encoded).
//   - Query parameters are stripped.
//   - Fragment is stripped.
//   - Trailing slashes are added to directory-like paths (paths ending in / or with no extension).
func Canonicalize(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// Strip query and fragment.
	u.RawQuery = ""
	u.Fragment = ""

	// Normalize percent-encoding: decode and re-encode path.
	decoded, err := url.PathUnescape(u.Path)
	if err == nil {
		// Re-encode only special characters.
		u.Path = encodePath(decoded)
	}

	return u.String()
}

// encodePath re-encodes a decoded path, preserving slashes.
func encodePath(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}
