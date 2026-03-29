package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const htmlBody = `<!DOCTYPE html>
<html><body>
<a href="../">Parent</a>
<a href="video.mp4">video.mp4</a>
<a href="subdir/">subdir/</a>
<a href="notes.txt">notes.txt</a>
<a href="random.xyz">random.xyz</a>
</body></html>`

func TestGenericHrefLister_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(htmlBody))
	}))
	defer srv.Close()

	l := GenericHrefLister{}
	entries, err := l.List(context.Background(), srv.URL+"/media/", srv.Client())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Should include video.mp4, subdir/, notes.txt; skip random.xyz and ../
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3: %v", len(entries), entries)
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	for _, want := range []string{"video.mp4", "subdir", "notes.txt"} {
		if !names[want] {
			t.Errorf("missing entry %q", want)
		}
	}
	if names["random.xyz"] {
		t.Error("random.xyz should have been filtered out")
	}
}

func TestGenericHrefLister_DirFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<a href="movies/">movies/</a>`))
	}))
	defer srv.Close()

	l := GenericHrefLister{}
	entries, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsDir {
		t.Errorf("expected 1 dir entry, got %+v", entries)
	}
}

func TestGenericHrefLister_NoMatch(t *testing.T) {
	// No recognizable hrefs at all.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><p>nothing here</p></body></html>`))
	}))
	defer srv.Close()

	l := GenericHrefLister{}
	_, err := l.List(context.Background(), srv.URL+"/", srv.Client())
	if err != ErrNoMatch {
		t.Errorf("want ErrNoMatch, got %v", err)
	}
}

func TestExtractHrefEntries_SameHostOnly(t *testing.T) {
	base, _ := url.Parse("https://media.example.com/")
	body := `<a href="https://evil.com/virus.mp4">evil</a><a href="movie.mp4">movie</a>`
	entries, _ := extractHrefEntries(body, base)
	if len(entries) != 1 || entries[0].Name != "movie.mp4" {
		t.Errorf("expected only same-host entry, got %+v", entries)
	}
}

func TestLastSegment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/a/b/c", "c"},
		{"/a/b/c/", "c"},
		{"/file.mp4", "file.mp4"},
		{"file.mp4", "file.mp4"},
	}
	for _, c := range cases {
		got := lastSegment(c.in)
		if got != c.want {
			t.Errorf("lastSegment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFileExt(t *testing.T) {
	cases := []struct{ in, want string }{
		{"video.mp4", "mp4"},
		{"VIDEO.MKV", "mkv"},
		{"noext", ""},
		{"a.b.tar.gz", "gz"},
	}
	for _, c := range cases {
		got := fileExt(c.in)
		if got != c.want {
			t.Errorf("fileExt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
