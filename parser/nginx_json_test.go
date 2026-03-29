package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const nginxJSONBody = `[
{"name":"video.mp4","type":"file","mtime":"Mon, 01 Jan 2024 12:00:00 GMT","size":10485760},
{"name":"subdir","type":"directory","mtime":"Mon, 01 Jan 2024 10:00:00 GMT","size":null},
{"name":"sub.srt","type":"file","mtime":"Mon, 01 Jan 2024 11:00:00 GMT","size":2048}
]`

func TestNginxJSONLister_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(nginxJSONBody))
	}))
	defer srv.Close()

	l := NginxJSONLister{}
	entries, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	// File entry.
	if entries[0].Name != "video.mp4" || entries[0].IsDir || entries[0].Size != 10485760 {
		t.Errorf("entry[0] unexpected: %+v", entries[0])
	}
	// Directory entry.
	if entries[1].Name != "subdir" || !entries[1].IsDir || entries[1].Size != -1 {
		t.Errorf("entry[1] unexpected: %+v", entries[1])
	}
	// URL construction.
	if entries[1].URL != srv.URL+"/subdir/" {
		t.Errorf("dir URL = %q, want %q", entries[1].URL, srv.URL+"/subdir/")
	}
}

func TestNginxJSONLister_HeuristicDetect(t *testing.T) {
	// No Content-Type header but body looks like nginx JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(nginxJSONBody))
	}))
	defer srv.Close()

	l := NginxJSONLister{}
	entries, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3", len(entries))
	}
}

func TestNginxJSONLister_NoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><a href='file.mp4'>file.mp4</a></body></html>"))
	}))
	defer srv.Close()

	l := NginxJSONLister{}
	_, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != ErrNoMatch {
		t.Errorf("want ErrNoMatch, got %v", err)
	}
}

func TestNginxJSONLister_EmptyArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	l := NginxJSONLister{}
	entries, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(entries))
	}
}

// Regression: joinURL must percent-encode path-unsafe characters in filenames.
// Filenames containing '#', '?', space, or a literal '%' must produce valid URLs
// that round-trip correctly. Without PathEscape, '#' and '?' truncate the URL at
// the fragment/query boundary; space produces an invalid URL; '%' double-encodes.
func TestJoinURL_SpecialChars(t *testing.T) {
	cases := []struct {
		name  string
		isDir bool
		want  string
	}{
		{"file #1.mp4", false, "http://x/file%20%231.mp4"},
		{"report?q=1.txt", false, "http://x/report%3Fq=1.txt"},
		{"my file.mkv", false, "http://x/my%20file.mkv"},
		{"50% done.srt", false, "http://x/50%25%20done.srt"},
		{"dir #1", true, "http://x/dir%20%231/"},
	}
	for _, c := range cases {
		got := joinURL("http://x", c.name, c.isDir)
		if got != c.want {
			t.Errorf("joinURL(%q, isDir=%v) = %q, want %q", c.name, c.isDir, got, c.want)
		}
	}
}
