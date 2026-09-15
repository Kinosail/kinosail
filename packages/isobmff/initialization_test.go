package isobmff

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func TestActualInitializationCodecAndColor(t *testing.T) { //nolint:cyclop // Each codec fixture checks its complete observed stream identity.
	for _, test := range []struct {
		codec, audio, hdr, want, videoRange string
		depth                               int
	}{
		{"h264", "aac", "", "avc1.64002A,mp4a.40.2", "SDR", 8},
		{"h264", "opus", "", "avc1.64002A,opus", "SDR", 8},
		{"hevc", "aac", "hdr10", "hvc1.2.6.L120.B0,mp4a.40.2", "PQ", 10},
		{"hevc", "", "hlg", "hvc1.2.6.L120.B0", "HLG", 10},
		{"av1", "", "", "av01.0.08M.08", "SDR", 8},
		{"vp9", "", "", "vp09.00.31.08", "SDR", 8},
	} {
		t.Run(test.want+test.hdr, func(t *testing.T) {
			actual, err := Parse(mp4fixture.Initialization(640, 360, test.codec, test.audio, test.hdr))
			video, found := actual.Video()
			if err != nil || !found || actual.Codecs() != test.want || video.Range != test.videoRange || video.BitDepth != test.depth || video.Width != 640 || video.Height != 360 {
				t.Fatalf("initialization = %#v, %v", actual, err)
			}
			if !HasCodec(actual, test.codec) || HasCodec(actual, "unknown") {
				t.Fatal("incorrect codec evidence")
			}
		})
	}
}

func TestInitializationRejectsMalformedAndOversizedInput(t *testing.T) {
	valid := mp4fixture.Initialization(640, 360, "h264", "aac", "")
	huge := make([]byte, MaximumInitializationBytes+1)
	unknown := bytes.ReplaceAll(valid, []byte("avc1"), []byte("nope"))
	oversizedBox := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(oversizedBox, 0xffffffff)
	nested := mp4fixture.Box("moov", valid)
	for range 12 {
		nested = mp4fixture.Box("moov", nested)
	}
	for _, data := range [][]byte{nil, []byte("init"), huge, unknown, oversizedBox, nested, valid[:len(valid)-1]} {
		if _, err := Parse(data); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted malformed initialization (%d bytes): %v", len(data), err)
		}
	}
	for length := 1; length < 8; length++ {
		if _, err := Parse(valid[:length]); err == nil {
			t.Fatal("accepted truncated header")
		}
	}
	path := filepath.Join(t.TempDir(), "init.mp4")
	if err := os.WriteFile(path, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	actual, err := Read(path)
	if err != nil || !strings.Contains(actual.Codecs(), "avc1.") {
		t.Fatalf("read = %#v, %v", actual, err)
	}
}

func TestInitializationKeepsDolbyCodecWhenConfigurationOrderChanges(t *testing.T) {
	entry := make([]byte, 78)
	binary.BigEndian.PutUint16(entry[24:], 1920)
	binary.BigEndian.PutUint16(entry[26:], 1080)
	entry = append(entry, mp4fixture.Box("dvcC", []byte{1, 0, 16, 48})...) //nolint:makezero // Configuration boxes follow the populated 78-byte visual sample header.
	configuration := make([]byte, 23)
	configuration[0] = 1
	entry = append(entry, mp4fixture.Box("hvcC", configuration)...) //nolint:makezero // Configuration boxes follow the populated 78-byte visual sample header.
	actual, err := Parse(mp4fixture.Box("moov", mp4fixture.Track("dvh1", entry)))
	if err != nil || actual.Codecs() != "dvh1.08.06" {
		t.Fatalf("Dolby configuration = %#v, %v", actual, err)
	}
}
