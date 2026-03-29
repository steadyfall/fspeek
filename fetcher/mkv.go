package fetcher

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	mkvparse "github.com/remko/go-mkvparse"
)

const mkvFetchSize = 512 * 1024 // 512KB

// MKVFetcher extracts metadata from MKV/WebM files.
type MKVFetcher struct{}

func (f MKVFetcher) Supports(ext string) bool {
	switch ext {
	case "mkv", "webm", "mka":
		return true
	}
	return false
}

func (f MKVFetcher) Fetch(ctx context.Context, url string, client *http.Client) (*Metadata, error) {
	data, err := FetchRange(ctx, client, url, 0, mkvFetchSize-1)
	if err != nil {
		return nil, err
	}

	h := &mkvHandler{}
	if err := mkvparse.ParseSections(bytes.NewReader(data), h,
		mkvparse.InfoElement,
		mkvparse.TracksElement,
	); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoMatch, err)
	}

	if !h.foundEBML {
		return nil, ErrNoMatch
	}

	meta := &Metadata{
		Format:       "Video / MKV",
		Duration:     h.duration,
		Codec:        h.videoCodec,
		AudioInfo:    h.audioInfo,
		RangeFetched: int64(len(data)),
	}
	return meta, nil
}

type mkvHandler struct {
	mkvparse.DefaultHandler

	foundEBML  bool
	duration   time.Duration
	videoCodec string
	audioInfo  string

	// per-track state
	inTrack      bool
	trackType    int64 // 1=video, 2=audio
	currentCodec string
	samplingFreq float64
	channels     int64
}

func (h *mkvHandler) HandleMasterBegin(id mkvparse.ElementID, info mkvparse.ElementInfo) (bool, error) {
	if id == mkvparse.SegmentElement || id == mkvparse.InfoElement || id == mkvparse.TracksElement {
		h.foundEBML = true
		return true, nil
	}
	if id == mkvparse.TrackEntryElement {
		h.inTrack = true
		h.trackType = 0
		h.currentCodec = ""
		h.samplingFreq = 0
		h.channels = 0
		return true, nil
	}
	if id == mkvparse.VideoElement || id == mkvparse.AudioElement {
		return true, nil
	}
	return true, nil
}

func (h *mkvHandler) HandleMasterEnd(id mkvparse.ElementID, info mkvparse.ElementInfo) error {
	if id == mkvparse.TrackEntryElement && h.inTrack {
		h.inTrack = false
		switch h.trackType {
		case 1: // video
			if h.videoCodec == "" && h.currentCodec != "" {
				h.videoCodec = normalizeVideoCodec(h.currentCodec)
			}
		case 2: // audio
			if h.audioInfo == "" {
				codec := normalizeAudioCodec(h.currentCodec)
				if h.channels > 0 && h.samplingFreq > 0 {
					h.audioInfo = fmt.Sprintf("%s, %.0f Hz, %d ch", codec, h.samplingFreq, h.channels)
				} else if h.channels > 0 {
					h.audioInfo = fmt.Sprintf("%s, %d ch", codec, h.channels)
				} else {
					h.audioInfo = codec
				}
			}
		}
	}
	return nil
}

func (h *mkvHandler) HandleFloat(id mkvparse.ElementID, value float64, info mkvparse.ElementInfo) error {
	switch id {
	case mkvparse.DurationElement:
		h.duration = time.Duration(value) * time.Millisecond
	case mkvparse.SamplingFrequencyElement:
		h.samplingFreq = value
	}
	return nil
}

func (h *mkvHandler) HandleInteger(id mkvparse.ElementID, value int64, info mkvparse.ElementInfo) error {
	switch id {
	case mkvparse.TrackTypeElement:
		h.trackType = value
	case mkvparse.ChannelsElement:
		h.channels = value
	}
	return nil
}

func (h *mkvHandler) HandleString(id mkvparse.ElementID, value string, info mkvparse.ElementInfo) error {
	if id == mkvparse.CodecIDElement {
		h.currentCodec = value
	}
	return nil
}

func normalizeVideoCodec(codecID string) string {
	switch codecID {
	case "V_MPEG4/ISO/AVC":
		return "H.264/AVC"
	case "V_MPEGH/ISO/HEVC":
		return "H.265/HEVC"
	case "V_VP8":
		return "VP8"
	case "V_VP9":
		return "VP9"
	case "V_AV1":
		return "AV1"
	default:
		return codecID
	}
}

func normalizeAudioCodec(codecID string) string {
	switch codecID {
	case "A_AAC":
		return "AAC"
	case "A_AC3":
		return "AC3"
	case "A_DTS":
		return "DTS"
	case "A_MPEG/L3":
		return "MP3"
	case "A_FLAC":
		return "FLAC"
	case "A_OPUS":
		return "Opus"
	case "A_VORBIS":
		return "Vorbis"
	default:
		return codecID
	}
}
