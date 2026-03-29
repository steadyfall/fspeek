package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	mp4 "github.com/abema/go-mp4"
)

// MP4Fetcher extracts metadata from MP4/M4V/M4A files.
type MP4Fetcher struct{}

func (f MP4Fetcher) Supports(ext string) bool {
	switch ext {
	case "mp4", "m4v", "m4a", "mov":
		return true
	}
	return false
}

func (f MP4Fetcher) Fetch(ctx context.Context, url string, client *http.Client) (*Metadata, error) {
	// HEAD to get Content-Length.
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	clStr := resp.Header.Get("Content-Length")
	if clStr == "" {
		return nil, ErrNoContentLength
	}
	contentLength, err := strconv.ParseInt(clStr, 10, 64)
	if err != nil || contentLength <= 0 {
		return nil, ErrNoContentLength
	}

	rs := &httpReadSeeker{
		ctx:           ctx,
		client:        client,
		url:           url,
		contentLength: contentLength,
	}

	info, err := mp4.Probe(rs)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoMatch, err)
	}

	meta := &Metadata{
		Format:       "Video / MP4",
		RangeFetched: rs.bytesRead,
	}

	// Movie-level duration.
	if info.Timescale > 0 && info.Duration > 0 {
		meta.Duration = time.Duration(info.Duration) * time.Second / time.Duration(info.Timescale)
	}

	for _, track := range info.Tracks {
		switch track.Codec {
		case mp4.CodecAVC1:
			meta.Codec = "H.264/AVC"
			if meta.Format == "Video / MP4" {
				meta.Format = "Video / MP4 (H.264)"
			}
		case mp4.CodecMP4A:
			channels := uint16(0)
			if track.MP4A != nil {
				channels = track.MP4A.ChannelCount
			}
			meta.AudioInfo = fmt.Sprintf("AAC, %d ch", channels)
		}
		// Use per-track duration if movie-level is missing.
		if meta.Duration == 0 && track.Timescale > 0 && track.Duration > 0 {
			meta.Duration = time.Duration(track.Duration) * time.Second / time.Duration(track.Timescale)
		}
	}

	return meta, nil
}

// httpReadSeeker implements io.ReadSeeker using HTTP range requests.
type httpReadSeeker struct {
	ctx           context.Context
	client        *http.Client
	url           string
	contentLength int64
	offset        int64
	bytesRead     int64
}

func (r *httpReadSeeker) Read(p []byte) (int, error) {
	if r.offset >= r.contentLength {
		return 0, io.EOF
	}
	end := r.offset + int64(len(p)) - 1
	if end >= r.contentLength {
		end = r.contentLength - 1
	}
	data, err := FetchRange(r.ctx, r.client, r.url, r.offset, end)
	if err != nil {
		return 0, err
	}
	n := copy(p, data)
	r.offset += int64(n)
	r.bytesRead += int64(n)
	return n, nil
}

func (r *httpReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.offset + offset
	case io.SeekEnd:
		if r.contentLength == 0 {
			return 0, ErrNoContentLength
		}
		newOffset = r.contentLength + offset
	default:
		return 0, fmt.Errorf("invalid whence %d", whence)
	}
	if newOffset < 0 {
		return 0, fmt.Errorf("negative seek position %d", newOffset)
	}
	r.offset = newOffset
	return r.offset, nil
}
