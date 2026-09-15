// Package isobmff reads the bounded codec configuration of generated MP4 files.
package isobmff

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
)

const MaximumInitializationBytes = 2 << 20

var ErrInvalid = errors.New("invalid or unsupported MP4 initialization")

type Track struct {
	Codec, Range  string
	Width, Height int
	BitDepth      int
}

type Initialization struct{ Tracks []Track }

func (initialization Initialization) Codecs() string {
	codecs := make([]string, 0, len(initialization.Tracks))
	for _, track := range initialization.Tracks {
		codecs = append(codecs, track.Codec)
	}
	return strings.Join(codecs, ",")
}

func (initialization Initialization) Video() (Track, bool) {
	for _, track := range initialization.Tracks {
		if track.Width > 0 {
			return track, true
		}
	}
	return Track{}, false
}

func Read(path string) (Initialization, error) {
	file, err := os.Open(path) //nolint:gosec // Caller supplies a generated output path.
	if err != nil {
		return Initialization{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaximumInitializationBytes+1))
	if err != nil {
		return Initialization{}, err
	}
	return Parse(data)
}

func Parse(data []byte) (Initialization, error) {
	if len(data) == 0 || len(data) > MaximumInitializationBytes {
		return Initialization{}, ErrInvalid
	}
	var result Initialization
	if err := walk(data, 0, &result); err != nil || len(result.Tracks) == 0 {
		return Initialization{}, ErrInvalid
	}
	return result, nil
}

func boxes(data []byte, visit func(string, []byte) error) error {
	for count := 0; len(data) > 0; count++ {
		if len(data) < 8 || count >= 4096 {
			return ErrInvalid
		}
		size, header, err := boxSize(data)
		if err != nil {
			return err
		}
		if err := visit(string(data[4:8]), data[header:size]); err != nil {
			return err
		}
		data = data[size:]
	}
	return nil
}

func walk(data []byte, depth int, result *Initialization) error {
	if depth > 8 {
		return ErrInvalid
	}
	return boxes(data, func(kind string, payload []byte) error {
		switch kind {
		case "trak":
			return walkTrack(payload, depth+1, result)
		case "moov", "mdia", "minf", "stbl":
			return walk(payload, depth+1, result)
		case "stsd":
			return readSampleDescription(payload, result)
		}
		return nil
	})
}

func walkTrack(data []byte, depth int, result *Initialization) error {
	var track Initialization
	if err := walk(data, depth, &track); err != nil {
		return err
	}
	rotate := false
	if err := boxes(data, func(kind string, payload []byte) error {
		if kind != "tkhd" {
			return nil
		}
		var err error
		rotate, err = trackRotation(payload)
		return err
	}); err != nil {
		return err
	}
	if len(result.Tracks)+len(track.Tracks) > 32 {
		return ErrInvalid
	}
	for _, value := range track.Tracks {
		if rotate && value.Width > 0 {
			value.Width, value.Height = value.Height, value.Width
		}
		result.Tracks = append(result.Tracks, value)
	}
	return nil
}

func trackRotation(payload []byte) (bool, error) {
	if len(payload) < 4 {
		return false, ErrInvalid
	}
	offset := 40
	if payload[0] == 1 {
		offset = 52
	} else if payload[0] != 0 {
		return false, ErrInvalid
	}
	if len(payload) < offset+36 {
		return false, ErrInvalid
	}
	a, b := binary.BigEndian.Uint32(payload[offset:]), binary.BigEndian.Uint32(payload[offset+4:])
	c, d := binary.BigEndian.Uint32(payload[offset+12:]), binary.BigEndian.Uint32(payload[offset+16:])
	// ISO BMFF stores this matrix as signed 16.16 fixed point.
	return a == 0 && d == 0 && (b == 0x00010000 && c == 0xffff0000 || b == 0xffff0000 && c == 0x00010000), nil
}

func boxSize(data []byte) (uint64, uint64, error) {
	size, header := uint64(binary.BigEndian.Uint32(data)), uint64(8)
	if size == 1 {
		if len(data) < 16 {
			return 0, 0, ErrInvalid
		}
		size, header = binary.BigEndian.Uint64(data[8:]), 16
	}
	if size == 0 {
		size = uint64(len(data))
	}
	if size < header || size > uint64(len(data)) {
		return 0, 0, ErrInvalid
	}
	return size, header, nil
}

func readSampleDescription(payload []byte, result *Initialization) error {
	if len(payload) < 8 || binary.BigEndian.Uint32(payload[4:]) != 1 || len(result.Tracks) >= 32 {
		return ErrInvalid
	}
	count := 0
	err := boxes(payload[8:], func(format string, entry []byte) error {
		count++
		if count != 1 {
			return ErrInvalid
		}
		track, err := sampleEntry(format, entry)
		if err != nil {
			return err
		}
		result.Tracks = append(result.Tracks, track)
		return nil
	})
	if err != nil || count != 1 {
		return ErrInvalid
	}
	return nil
}
