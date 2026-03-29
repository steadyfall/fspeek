package fetcher

import (
	"testing"
)

func TestMKVFetcher_Supports(t *testing.T) {
	f := MKVFetcher{}
	for _, ext := range []string{"mkv", "webm", "mka"} {
		if !f.Supports(ext) {
			t.Errorf("Supports(%q) = false, want true", ext)
		}
	}
	if f.Supports("mp4") {
		t.Error("Supports(mp4) = true, want false")
	}
}

func TestNormalizeVideoCodec(t *testing.T) {
	cases := []struct{ in, want string }{
		{"V_MPEG4/ISO/AVC", "H.264/AVC"},
		{"V_MPEGH/ISO/HEVC", "H.265/HEVC"},
		{"V_VP9", "VP9"},
		{"V_UNKNOWN", "V_UNKNOWN"},
	}
	for _, c := range cases {
		got := normalizeVideoCodec(c.in)
		if got != c.want {
			t.Errorf("normalizeVideoCodec(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeAudioCodec(t *testing.T) {
	cases := []struct{ in, want string }{
		{"A_AAC", "AAC"},
		{"A_AC3", "AC3"},
		{"A_FLAC", "FLAC"},
		{"A_OPUS", "Opus"},
		{"A_UNKNOWN", "A_UNKNOWN"},
	}
	for _, c := range cases {
		got := normalizeAudioCodec(c.in)
		if got != c.want {
			t.Errorf("normalizeAudioCodec(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
