// Package mp4fixture builds sample-description fixtures, not playable media.
// It is used by mocked FFmpeg tests; real media checks must run an encoder.
package mp4fixture

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

func Box(kind string, payload []byte) []byte {
	if len(payload) > math.MaxUint32-8 {
		panic("MP4 fixture box is too large")
	}
	result := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(result, uint32(len(result))) //nolint:gosec // The payload length is bounded to MaxUint32-8 before allocation above.
	copy(result[4:8], kind)
	copy(result[8:], payload)
	return result
}

func Track(format string, entry []byte) []byte {
	description := append([]byte{0, 0, 0, 0, 0, 0, 0, 1}, Box(format, entry)...)
	return Box("trak", Box("mdia", Box("minf", Box("stbl", Box("stsd", description)))))
}

func Initialization(width, height int, codec, audio, hdr string) []byte { //nolint:cyclop // This fixture enumerates codec-specific sample-description headers.
	if width < 0 || width > math.MaxUint16 || height < 0 || height > math.MaxUint16 {
		panic("MP4 fixture dimensions are out of range")
	}
	entry := make([]byte, 78)
	binary.BigEndian.PutUint16(entry[24:], uint16(width))
	binary.BigEndian.PutUint16(entry[26:], uint16(height))
	format, configuration := "avc1", Box("avcC", []byte{1, 100, 0, 42, 255, 225, 0, 2, 0x67, 0, 1, 0, 2, 0x68, 0})
	switch codec {
	case "hevc":
		format = "hvc1"
		config := make([]byte, 23)
		config[0], config[1], config[2], config[6], config[12] = 1, 1, 0x60, 0xb0, 120
		if hdr != "" {
			config[1], config[17], config[18] = 2, 2, 2
		}
		configuration = Box("hvcC", config)
	case "av1":
		format, configuration = "av01", Box("av1C", []byte{0x81, 8, 0, 0})
	case "vp9":
		format, configuration = "vp09", Box("vpcC", []byte{1, 0, 0, 0, 0, 31, 0x80, 1, 1, 1, 0, 0})
	}
	entry = append(entry, configuration...) //nolint:makezero // Codec boxes follow the populated 78-byte visual sample header.
	if hdr != "" {
		transfer := byte(16)
		if hdr == "hlg" {
			transfer = 18
		}
		entry = append(entry, Box("colr", []byte{'n', 'c', 'l', 'x', 0, 9, 0, transfer, 0, 9, 0})...) //nolint:makezero // Color boxes follow the populated visual sample header.
	}
	tracks := Track(format, entry)
	if audio != "" {
		entry := make([]byte, 28)
		format := "mp4a"
		if audio == "opus" {
			format = "Opus"
		} else {
			decoder := append([]byte{0x40, 0x15, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte{5, 2, 0x12, 0x10}...)
			es := append([]byte{0, 1, 0}, descriptor(4, decoder)...)
			entry = append(entry, Box("esds", append([]byte{0, 0, 0, 0}, descriptor(3, es)...))...) //nolint:makezero // Descriptor boxes follow the required 28-byte audio header.
		}
		tracks = append(tracks, Track(format, entry)...)
	}
	return Box("moov", tracks)
}

func Shell(data []byte) string {
	var result strings.Builder
	result.WriteString("printf '")
	for _, value := range data {
		fmt.Fprintf(&result, "\\%03o", value)
	}
	result.WriteString("'")
	return result.String()
}

func descriptor(tag byte, payload []byte) []byte {
	if len(payload) > 127 {
		panic("MP4 fixture descriptor is too large")
	}
	return append([]byte{tag, byte(len(payload))}, payload...) //nolint:gosec // The one-byte descriptor length is bounded to 127 above.
}
