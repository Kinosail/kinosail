package isobmff

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func geometryInitialization(aspects [][]byte, header []byte) []byte {
	entry := make([]byte, 78, 256)
	binary.BigEndian.PutUint16(entry[24:], 640)
	binary.BigEndian.PutUint16(entry[26:], 360)
	entry = append(entry, mp4fixture.Box("avcC", []byte{1, 100, 0, 42, 255, 225, 0})...) //nolint:makezero // The first 78 bytes are the required visual sample-entry header.
	for _, aspect := range aspects {
		entry = append(entry, mp4fixture.Box("pasp", aspect)...)
	} //nolint:makezero // Append boxes after the populated sample-entry header.
	track := mp4fixture.Track("avc1", entry)
	return mp4fixture.Box("moov", mp4fixture.Box("trak", append(track[8:], mp4fixture.Box("tkhd", header)...)))
}

func aspectRatio(numerator, denominator uint32) []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data, numerator)
	binary.BigEndian.PutUint32(data[4:], denominator)
	return data
}

func TestGeometryRejectsMalformedAndOverflowingAspectWithoutResult(t *testing.T) {
	header := make([]byte, 76)
	for _, aspects := range [][][]byte{
		{nil},
		{{1}},
		{aspectRatio(0, 1)},
		{aspectRatio(1, 0)},
		{aspectRatio(10001, 1)},
		{aspectRatio(1, 10001)},
		{aspectRatio(10000, 1), aspectRatio(10000, 1)},
	} {
		result, err := Parse(geometryInitialization(aspects, header))
		if !errors.Is(err, ErrInvalid) || len(result.Tracks) != 0 {
			t.Fatalf("invalid geometry returned tracks: %#v, %v", result, err)
		}
	}
}

func TestGeometryPreservesAspectAndSignedQuarterTurns(t *testing.T) {
	for _, version := range []byte{0, 1} {
		for _, clockwise := range []bool{false, true} {
			header := quarterTurnHeader(version, clockwise)
			result, err := Parse(geometryInitialization([][]byte{aspectRatio(2, 1)}, header))
			if err != nil {
				t.Fatal(err)
			}
			video, found := result.Video()
			if !found || video.Width != 360 || video.Height != 1280 {
				t.Fatalf("geometry: %#v", result)
			}
		}
	}
}

func TestGeometryRejectsUnknownAndTruncatedTrackHeaders(t *testing.T) {
	unknown := make([]byte, 88)
	unknown[0] = 2
	for _, header := range [][]byte{nil, {0}, make([]byte, 75), append([]byte{1}, make([]byte, 86)...), unknown} {
		if result, err := Parse(geometryInitialization(nil, header)); !errors.Is(err, ErrInvalid) || len(result.Tracks) != 0 {
			t.Fatalf("accepted malformed track header: %#v, %v", result, err)
		}
	}
}

func quarterTurnHeader(version byte, clockwise bool) []byte {
	header := make([]byte, 88)
	header[0] = version
	offset := 40
	if version == 1 {
		offset = 52
	}
	b, c := uint32(0x00010000), uint32(0xffff0000)
	if clockwise {
		b, c = c, b
	}
	binary.BigEndian.PutUint32(header[offset+4:], b)
	binary.BigEndian.PutUint32(header[offset+12:], c)
	return header
}
