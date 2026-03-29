package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRange_206(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "" {
			t.Error("Range header missing")
		}
		w.Header().Set("Content-Range", "bytes 0-4/10")
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	data, err := FetchRange(context.Background(), srv.Client(), srv.URL, 0, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", data, "hello")
	}
}

func TestFetchRange_200ReturnsErrRangeIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("full file"))
	}))
	defer srv.Close()

	_, err := FetchRange(context.Background(), srv.Client(), srv.URL, 0, 4)
	if err != ErrRangeIgnored {
		t.Errorf("want ErrRangeIgnored, got %v", err)
	}
}
