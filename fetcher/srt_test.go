package fetcher

import (
	"testing"
	"time"
)

func TestSRTFetcher_Supports(t *testing.T) {
	f := SRTFetcher{}
	if !f.Supports("srt") {
		t.Error("Supports(srt) = false, want true")
	}
	if f.Supports("mp4") {
		t.Error("Supports(mp4) = true, want false")
	}
}

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"01:23:45,678", time.Hour + 23*time.Minute + 45*time.Second + 678*time.Millisecond, true},
		{"00:00:01,000", time.Second, true},
		{"bad", 0, false},
	}
	for _, c := range cases {
		got, ok := parseTimestamp(c.in)
		if ok != c.ok {
			t.Errorf("parseTimestamp(%q) ok=%v, want %v", c.in, ok, c.ok)
		}
		if ok && got != c.want {
			t.Errorf("parseTimestamp(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestLastTimestamp(t *testing.T) {
	srt := []byte(`1
00:00:01,000 --> 00:00:03,000
Hello world

2
00:01:30,500 --> 00:01:32,000
Goodbye
`)
	d := lastTimestamp(srt)
	want := time.Minute + 32*time.Second
	if d != want {
		t.Errorf("lastTimestamp = %v, want %v", d, want)
	}
}

func TestFormatDuration(t *testing.T) {
	d := 2*time.Hour + 3*time.Minute + 4*time.Second
	got := FormatDuration(d)
	if got != "02:03:04" {
		t.Errorf("FormatDuration = %q, want %q", got, "02:03:04")
	}
}
