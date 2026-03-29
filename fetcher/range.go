package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// FetchRange fetches bytes [start, end] (inclusive) from url via HTTP range request.
// Returns ErrRangeIgnored if the server responds with 200 instead of 206.
func FetchRange(ctx context.Context, client *http.Client, url string, start, end int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil, ErrRangeIgnored
	}
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
