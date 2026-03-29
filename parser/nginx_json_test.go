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
