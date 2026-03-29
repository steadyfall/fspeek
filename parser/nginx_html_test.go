package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const nginxHTMLBody = `<!DOCTYPE html>
<html>
<head><title>Index of /media/</title></head>
<body bgcolor="white">
<h1>Index of /media/</h1><hr><pre><a href="../">../</a>
<a href="subdir/">subdir/</a>                                        01-Jan-2024 10:00       -
<a href="video.mp4">video.mp4</a>                                   01-Jan-2024 12:00   524288000
<a href="sub.srt">sub.srt</a>                                       01-Jan-2024 11:00        2048
</pre><hr></body>
</html>`

func TestNginxHTMLLister_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(nginxHTMLBody))
	}))
	defer srv.Close()

	l := NginxHTMLLister{}
	entries, err := l.List(context.Background(), srv.URL+"/media/", srv.Client())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3: %v", len(entries), entries)
	}

	byName := map[string]int{}
	for i, e := range entries {
		byName[e.Name] = i
	}

	// Directory entry.
	si, ok := byName["subdir"]
	if !ok {
		t.Fatal("missing subdir entry")
	}
	if !entries[si].IsDir {
		t.Error("subdir: IsDir should be true")
	}
	if entries[si].Size != -1 {
		t.Errorf("subdir: Size = %d, want -1", entries[si].Size)
	}

	// File entry with size.
	vi, ok := byName["video.mp4"]
	if !ok {
		t.Fatal("missing video.mp4 entry")
	}
	if entries[vi].IsDir {
		t.Error("video.mp4: IsDir should be false")
	}
	if entries[vi].Size != 524288000 {
		t.Errorf("video.mp4: Size = %d, want 524288000", entries[vi].Size)
	}
	if entries[vi].ModTime.IsZero() {
		t.Error("video.mp4: ModTime should not be zero")
	}

	// Another file entry.
	ri, ok := byName["sub.srt"]
	if !ok {
		t.Fatal("missing sub.srt entry")
	}
	if entries[ri].Size != 2048 {
		t.Errorf("sub.srt: Size = %d, want 2048", entries[ri].Size)
	}
}

func TestNginxHTMLLister_NoMatch_PlainHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><a href="video.mp4">video.mp4</a></body></html>`))
	}))
	defer srv.Close()

	l := NginxHTMLLister{}
	_, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != ErrNoMatch {
		t.Errorf("want ErrNoMatch, got %v", err)
	}
}

func TestNginxHTMLLister_NoMatch_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(nginxJSONBody))
	}))
	defer srv.Close()

	l := NginxHTMLLister{}
	_, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != ErrNoMatch {
		t.Errorf("want ErrNoMatch, got %v", err)
	}
}

func TestNginxHTMLLister_SkipsUnknownExtensions(t *testing.T) {
	body := `<!DOCTYPE html><html><body><pre>
<a href="random.xyz">random.xyz</a>              01-Jan-2024 12:00        1234
<a href="video.mp4">video.mp4</a>                01-Jan-2024 12:00   524288000
</pre></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	l := NginxHTMLLister{}
	entries, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "video.mp4" {
		t.Errorf("expected only video.mp4, got %+v", entries)
	}
}
