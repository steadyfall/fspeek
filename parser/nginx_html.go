package parser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/steadyfall/fspeek/cache"
	"golang.org/x/net/html"
)

// nginxHTMLEntry regexp matches the date+size suffix on each nginx autoindex line:
//
//	"01-Jan-2024 12:00   524288000"  (file)
//	"01-Jan-2024 10:00           -"  (directory)
var nginxLineSuffix = regexp.MustCompile(`(\d{2}-\w{3}-\d{4} \d{2}:\d{2})\s+(\d+|-)`)

// NginxHTMLLister parses the standard nginx autoindex HTML output (autoindex on;
// without autoindex_format json). It extracts file sizes and mod times from the
// text nodes that follow each <a> link inside the <pre> block.
type NginxHTMLLister struct{}

func (l NginxHTMLLister) List(ctx context.Context, dirURL string, client *http.Client) ([]cache.Entry, error) {
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

	entries, ok := extractNginxHTMLEntries(string(body), base)
	if !ok {
		return nil, ErrNoMatch
	}
	return entries, nil
}

// extractNginxHTMLEntries parses nginx autoindex HTML and returns entries with
// sizes and mod times. ok is false if the page does not look like nginx autoindex.
func extractNginxHTMLEntries(body string, base *url.URL) ([]cache.Entry, bool) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, false
	}

	var entries []cache.Entry
	foundNginxPattern := false

	var walkPre func(*html.Node)
	walkPre = func(pre *html.Node) {
		// Iterate children of <pre>. For each <a> node, the immediately following
		// text sibling contains the date and size.
		for child := pre.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode || child.Data != "a" {
				continue
			}

			href := attrVal(child.Attr, "href")
			if href == "" || href == "/" || href == ".." || href == "../" ||
				strings.HasPrefix(href, "?") || strings.HasPrefix(href, "#") {
				continue
			}

			ref, err := url.Parse(href)
			if err != nil {
				continue
			}
			abs := base.ResolveReference(ref)
			if abs.Host != base.Host {
				continue
			}

			name := lastSegment(abs.Path)
			if name == "" {
				continue
			}
			isDir := strings.HasSuffix(abs.Path, "/")
			if !isDir && !allowedExts[fileExt(name)] {
				continue
			}

			// The text sibling immediately after </a> holds "  DD-Mon-YYYY HH:MM  SIZE".
			var size int64 = -1
			var modTime time.Time

			if text := child.NextSibling; text != nil && text.Type == html.TextNode {
				if m := nginxLineSuffix.FindStringSubmatch(text.Data); m != nil {
					foundNginxPattern = true
					modTime, _ = time.Parse("02-Jan-2006 15:04", m[1])
					if m[2] != "-" {
						if n, err := strconv.ParseInt(m[2], 10, 64); err == nil {
							size = n
						}
					}
				}
			}

			entries = append(entries, cache.Entry{
				Name:    name,
				URL:     abs.String(),
				IsDir:   isDir,
				Size:    size,
				ModTime: modTime,
			})
		}
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "pre" {
			walkPre(n)
			return // don't recurse into <pre>
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if !foundNginxPattern || len(entries) == 0 {
		return nil, false
	}
	return entries, true
}
