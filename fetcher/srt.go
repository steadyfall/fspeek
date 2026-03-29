package fetcher

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	srtHeadSize = 4 * 1024  // 4KB for format detection
	srtTailSize = 4 * 1024  // 4KB to find last timestamp
)

var (
	// SubRip format: sequence number, then timestamp line.
	srtSeqRe       = regexp.MustCompile(`(?m)^\d+\r?\n\d{2}:\d{2}`)
	srtTimestampRe = regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2}),(\d{3})`)
)

// SRTFetcher extracts metadata from SubRip subtitle files.
type SRTFetcher struct{}

func (f SRTFetcher) Supports(ext string) bool {
	return ext == "srt"
}

func (f SRTFetcher) Fetch(ctx context.Context, url string, client *http.Client) (*Metadata, error) {
	head, err := FetchRange(ctx, client, url, 0, srtHeadSize-1)
	if err != nil {
		return nil, err
	}

	if !srtSeqRe.Match(head) {
		return nil, ErrNoMatch
	}

	meta := &Metadata{
		Format:       "Subtitles / SRT",
		RangeFetched: int64(len(head)),
	}

	// Try to get Content-Length for tail fetch.
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return meta, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return meta, nil
	}
	resp.Body.Close()

	clStr := resp.Header.Get("Content-Length")
	if clStr == "" {
		return meta, nil
	}
	contentLength, err := strconv.ParseInt(clStr, 10, 64)
	if err != nil || contentLength <= srtHeadSize {
		// Duration from head only.
		if d := lastTimestamp(head); d > 0 {
			meta.Duration = d
		}
		return meta, nil
	}

	tailStart := contentLength - srtTailSize
	if tailStart < 0 {
		tailStart = 0
	}
	tail, err := FetchRange(ctx, client, url, tailStart, contentLength-1)
	if err != nil {
		// Non-fatal: return without duration.
		return meta, nil
	}
	meta.RangeFetched += int64(len(tail))

	if d := lastTimestamp(tail); d > 0 {
		meta.Duration = d
	}
	return meta, nil
}

// lastTimestamp finds the last SRT timestamp in data and returns its end time.
func lastTimestamp(data []byte) time.Duration {
	// Find all lines with " --> " pattern (timestamp lines).
	lines := bytes.Split(data, []byte("\n"))
	var last time.Duration
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if !bytes.Contains(line, []byte(" --> ")) {
			continue
		}
		parts := strings.Split(string(line), " --> ")
		if len(parts) < 2 {
			continue
		}
		// Use the end time (second part).
		if d, ok := parseTimestamp(strings.TrimSpace(parts[1])); ok {
			last = d
		}
	}
	return last
}

// parseTimestamp parses "HH:MM:SS,mmm" into time.Duration.
func parseTimestamp(s string) (time.Duration, bool) {
	m := srtTimestampRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	ms, _ := strconv.Atoi(m[4])
	d := time.Duration(h)*time.Hour +
		time.Duration(min)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(ms)*time.Millisecond
	return d, true
}

// FormatDuration formats a duration as "HH:MM:SS".
func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
