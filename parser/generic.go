package parser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/steadyfall/fspeek/cache"
	"golang.org/x/net/html"
)

// allowedExts is the set of file extensions GenericHrefLister will include.
var allowedExts = map[string]bool{
	"mp4": true, "m4v": true, "m4a": true, "mov": true,
	"mkv": true, "webm": true, "mka": true,
	"srt": true, "vtt": true, "ass": true, "ssa": true,
	"mp3": true, "flac": true, "ogg": true, "opus": true, "wav": true, "aac": true,
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true,
	"pdf": true, "txt": true, "nfo": true,
	"zip": true, "tar": true, "gz": true, "bz2": true, "xz": true, "rar": true, "7z": true,
	"iso": true, "img": true,
}

// GenericHrefLister parses any HTML page and extracts href links that look like
// directory entries. It recognises directories (hrefs ending in "/") and files
// matching the allowed extension list.
type GenericHrefLister struct{}

func (l GenericHrefLister) List(ctx context.Context, dirURL string, client *http.Client) ([]cache.Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dirURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") && ct != "" {
		return nil, ErrNoMatch
	}

	const maxBody = 4 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(dirURL)
	if err != nil {
		return nil, err
	}

	entries, recognized := extractHrefEntries(string(body), base)
	if !recognized {
		return nil, ErrNoMatch
	}
	return entries, nil
}

// extractHrefEntries scans HTML for <a href> links and returns matching entries.
// recognized is true if at least one directory or allowlisted-extension link was found.
func extractHrefEntries(body string, base *url.URL) ([]cache.Entry, bool) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, false
	}

	var entries []cache.Entry
	seen := map[string]bool{}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attrVal(n.Attr, "href")
			if href == "" || href == "/" || href == ".." || href == "../" ||
				strings.HasPrefix(href, "?") || strings.HasPrefix(href, "#") {
				goto children
			}

			{
				ref, err := url.Parse(href)
				if err != nil {
					goto children
				}
				abs := base.ResolveReference(ref)

				// Only follow same-host links.
				if abs.Host != base.Host {
					goto children
				}

				name := lastSegment(abs.Path)
				if name == "" {
					goto children
				}

				isDir := strings.HasSuffix(abs.Path, "/")
				isAllowed := isDir || allowedExts[fileExt(name)]
				if !isAllowed {
					goto children
				}

				entryURL := abs.String()
				if seen[entryURL] {
					goto children
				}
				seen[entryURL] = true

				entries = append(entries, cache.Entry{
					Name:  name,
					URL:   entryURL,
					IsDir: isDir,
					Size:  -1,
				})
			}
		}
	children:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return entries, len(entries) > 0
}

func attrVal(attrs []html.Attribute, key string) string { //nolint:unparam
	for _, a := range attrs {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// lastSegment returns the last non-empty path segment (without trailing slash).
func lastSegment(p string) string {
	p = strings.TrimRight(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}

// fileExt returns the lower-case extension of a filename without the dot.
func fileExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(name[idx+1:])
}
