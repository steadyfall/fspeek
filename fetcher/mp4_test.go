package fetcher

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMP4Fetcher_Supports(t *testing.T) {
	f := MP4Fetcher{}
	for _, ext := range []string{"mp4", "m4v", "m4a", "mov"} {
		if !f.Supports(ext) {
			t.Errorf("Supports(%q) = false, want true", ext)
		}
	}
	if f.Supports("mkv") {
		t.Error("Supports(mkv) = true, want false")
	}
}

func TestMP4Fetcher_NoContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := MP4Fetcher{}
	_, err := f.Fetch(context.Background(), srv.URL+"/test.mp4", srv.Client())
	if err != ErrNoContentLength {
		t.Errorf("want ErrNoContentLength, got %v", err)
	}
}

func TestHttpReadSeeker_ReadAndSeek(t *testing.T) {
	data := []byte("0123456789abcdefghij")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "test.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()

	rs := &httpReadSeeker{
		ctx:           context.Background(),
		client:        srv.Client(),
		url:           srv.URL + "/test.bin",
		contentLength: int64(len(data)),
	}

	// Read 4 bytes from start.
	p := make([]byte, 4)
	n, err := rs.Read(p)
	if err != nil && err != io.EOF {
		t.Fatalf("Read: %v", err)
	}
	if n != 4 || string(p[:n]) != "0123" {
		t.Errorf("Read got %q, want %q", p[:n], "0123")
	}

	// Seek to end - 5, read remaining.
	pos, err := rs.Seek(-5, io.SeekEnd)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}
	want := int64(len(data)) - 5
	if pos != want {
		t.Errorf("Seek pos = %d, want %d", pos, want)
	}

	rest := make([]byte, 5)
	n, err = rs.Read(rest)
	if err != nil && err != io.EOF {
		t.Fatalf("Read after seek: %v", err)
	}
	if string(rest[:n]) != "fghij" {
		t.Errorf("Read after seek got %q, want %q", rest[:n], "fghij")
	}
}
