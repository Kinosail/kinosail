package isobmff

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
)

func codecConfiguration(format, kind string, data []byte) (string, error) {
	switch kind {
	case "avcC":
		return avcCodec(format, data)
	case "hvcC":
		return hevcCodec(format, data)
	case "av1C":
		return av1Codec(data)
	case "vpcC":
		return vp9Codec(data)
	case "dvcC", "dvvC":
		return dolbyCodec(format, data)
	case "esds":
		return esdsCodec(data)
	}
	return "", nil
}

func avcCodec(format string, data []byte) (string, error) {
	if len(data) < 7 || data[0] != 1 {
		return "", ErrInvalid
	}
	return fmt.Sprintf("%s.%02X%02X%02X", format, data[1], data[2], data[3]), nil
}

func hevcCodec(format string, data []byte) (string, error) {
	if format == "dvh1" || format == "dvhe" {
		return "", nil
	}
	if len(data) < 23 || data[0] != 1 {
		return "", ErrInvalid
	}
	space := []string{"", "A", "B", "C"}[data[1]>>6]
	tier := "L"
	if data[1]&32 != 0 {
		tier = "H"
	}
	codec := fmt.Sprintf("%s.%s%d.%X.%s%d", format, space, data[1]&31, bits.Reverse32(binary.BigEndian.Uint32(data[2:])), tier, data[12])
	end := 12
	for end > 6 && data[end-1] == 0 {
		end--
	}
	var constraints strings.Builder
	for _, constraint := range data[6:end] {
		fmt.Fprintf(&constraints, ".%02X", constraint)
	}
	codec += constraints.String()
	return codec, nil
}

func av1Codec(data []byte) (string, error) {
	if len(data) < 4 || data[0] != 0x81 {
		return "", ErrInvalid
	}
	depth, tier := av1BitDepth(data[2]), "M"
	if data[2]&128 != 0 {
		tier = "H"
	}
	return fmt.Sprintf("av01.%d.%02d%s.%02d", data[1]>>5, data[1]&31, tier, depth), nil
}

func vp9Codec(data []byte) (string, error) {
	if len(data) < 12 {
		return "", ErrInvalid
	}
	return fmt.Sprintf("vp09.%02d.%02d.%02d", data[4], data[5], data[6]>>4), nil
}

func dolbyCodec(format string, data []byte) (string, error) {
	if len(data) < 4 {
		return "", ErrInvalid
	}
	if format != "dvh1" && format != "dvhe" {
		return "", nil
	}
	return fmt.Sprintf("%s.%02d.%02d", format, data[2]>>1, (data[2]&1)<<5|data[3]>>3), nil
}

func esdsCodec(data []byte) (string, error) {
	if len(data) < 4 {
		return "", ErrInvalid
	}
	return audioDescriptor(data[4:])
}

func descriptor(data []byte) (byte, []byte, error) {
	if len(data) < 2 {
		return 0, nil, ErrInvalid
	}
	tag, length, offset := data[0], 0, 1
	for count := 0; count < 4; count++ {
		if offset >= len(data) {
			return 0, nil, ErrInvalid
		}
		value := data[offset]
		offset++
		length = length<<7 | int(value&127)
		if value&128 == 0 {
			if length > len(data)-offset {
				return 0, nil, ErrInvalid
			}
			return tag, data[offset : offset+length], nil
		}
	}
	return 0, nil, ErrInvalid
}

func audioDescriptor(data []byte) (string, error) { //nolint:cyclop // Validate nested descriptor headers before reading the audio object type.
	tag, payload, err := descriptor(data)
	if err != nil || tag != 3 || len(payload) < 3 {
		return "", ErrInvalid
	}
	offset, err := audioDescriptorOffset(payload)
	if err != nil {
		return "", err
	}
	tag, payload, err = descriptor(payload[offset:])
	if err != nil || tag != 4 || len(payload) < 13 {
		return "", ErrInvalid
	}
	object := payload[0]
	if object == 0x6b || object == 0x69 {
		return fmt.Sprintf("mp4a.%02X", object), nil
	}
	if object != 0x40 {
		return "", ErrInvalid
	}
	return aacDescriptor(payload[13:])
}

func HasCodec(initialization Initialization, codec string) bool {
	prefix := map[string]string{"h264": "avc", "hevc": "hv", "av1": "av01", "vp9": "vp09"}[codec]
	video, found := initialization.Video()
	return found && prefix != "" && strings.HasPrefix(video.Codec, prefix)
}

func av1BitDepth(flags byte) int {
	if flags&32 != 0 {
		return 12
	}
	if flags&64 != 0 {
		return 10
	}
	return 8
}

func audioDescriptorOffset(payload []byte) (int, error) {
	offset, flags := 3, payload[2]
	if flags&128 != 0 {
		offset += 2
	}
	if flags&64 != 0 {
		if offset >= len(payload) {
			return 0, ErrInvalid
		}
		offset += 1 + int(payload[offset])
	}
	if flags&32 != 0 {
		offset += 2
	}
	if offset > len(payload) {
		return 0, ErrInvalid
	}
	return offset, nil
}

func aacDescriptor(data []byte) (string, error) {
	tag, config, err := descriptor(data)
	if err != nil || tag != 5 || len(config) < 2 {
		return "", ErrInvalid
	}
	audioType := int(config[0] >> 3)
	if audioType == 31 {
		audioType = 32 + (int(config[0]&7) << 3) + int(config[1]>>5)
	}
	if audioType == 0 {
		return "", ErrInvalid
	}
	return "mp4a.40." + strconv.Itoa(audioType), nil
}
