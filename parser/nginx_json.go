package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/steadyfall/fspeek/cache"
)

// nginxEntry matches the JSON schema emitted by nginx autoindex_format json.
type nginxEntry struct {
	Name  string `json:"name"`
	Type  string `json:"type"` // "file" or "directory"
	Mtime string `json:"mtime"`
	Size  *int64 `json:"size"` // null for directories
}

// NginxJSONLister parses nginx autoindex JSON output.
type NginxJSONLister struct{}

func (l NginxJSONLister) List(ctx context.Context, dirURL string, client *http.Client) ([]cache.Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dirURL, nil)
	if err != nil {
		return nil, err
	}
	// Prefer JSON if the server does content negotiation.
	req.Header.Set("Accept", "application/json, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	isJSON := strings.Contains(ct, "application/json")

	// Read a small prefix to detect the JSON array pattern.
	const maxBody = 4 * 1024 * 1024 // 4 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(body))
	if !isJSON {
		// Heuristic: nginx JSON output starts with '[' (array).
		if !strings.HasPrefix(trimmed, "[") {
			return nil, ErrNoMatch
		}
		// Must contain the nginx JSON shape: {"name": ...}
		if !strings.Contains(trimmed, `"name"`) {
			return nil, ErrNoMatch
		}
	}

	var raw []nginxEntry
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, ErrNoMatch
	}

	entries := make([]cache.Entry, 0, len(raw))
	for _, r := range raw {
		if r.Name == "" {
			continue
		}
		isDir := r.Type == "directory"
		size := int64(-1)
		if r.Size != nil {
			size = *r.Size
		}
		entryURL := joinURL(dirURL, r.Name, isDir)
		modTime, _ := parseNginxTime(r.Mtime)
		entries = append(entries, cache.Entry{
			Name:    r.Name,
			URL:     entryURL,
			IsDir:   isDir,
			Size:    size,
			ModTime: modTime,
		})
	}
	return entries, nil
}

// parseNginxTime parses nginx mtime format: "Thu, 01 Jan 2015 12:00:00 GMT"
func parseNginxTime(s string) (time.Time, error) {
	return time.Parse("Mon, 02 Jan 2006 15:04:05 MST", s)
}

// joinURL appends name to the base directory URL.
func joinURL(base, name string, isDir bool) string {
	base = strings.TrimRight(base, "/")
	if isDir {
		return base + "/" + name + "/"
	}
	return base + "/" + name
}
