package fetcher

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"
)

// Typed sentinel errors.
var (
	ErrRangeUnsupported = errors.New("range requests not supported")
	ErrNoContentLength  = errors.New("Content-Length header absent")
	ErrNoMatch          = errors.New("fetcher does not handle this format")
	ErrRangeIgnored     = errors.New("server returned 200 instead of 206")
)

// Metadata holds extracted file metadata.
type Metadata struct {
	Format       string
	Duration     time.Duration
	Codec        string
	AudioInfo    string
	Extra        map[string]string
	RangeFetched int64
}

// MetadataFetcher is the interface all format-specific fetchers implement.
type MetadataFetcher interface {
	Supports(ext string) bool
	Fetch(ctx context.Context, url string, client *http.Client) (*Metadata, error)
}

// Dispatch issues a HEAD request to check Accept-Ranges, then dispatches to
// the first fetcher that Supports the file extension.
func Dispatch(ctx context.Context, url string, client *http.Client, fetchers []MetadataFetcher) (*Metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	ar := strings.ToLower(resp.Header.Get("Accept-Ranges"))
	if ar == "none" || ar == "" {
		return nil, ErrRangeUnsupported
	}

	ext := strings.ToLower(strings.TrimPrefix(path.Ext(url), "."))
	for _, f := range fetchers {
		if f.Supports(ext) {
			return f.Fetch(ctx, url, client)
		}
	}
	return nil, ErrNoMatch
}
