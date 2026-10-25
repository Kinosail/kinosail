package playback

import (
	"encoding/binary"
	"errors"
)

const maximumIFrameFileBytes = 1 << 20

var errIFrameContainer = errors.New("I-frame fragment is invalid")

type mp4Box struct {
	kind              string
	start, body, ends int
}

func mp4Boxes(data []byte, start, end int) ([]mp4Box, error) {
	var boxes []mp4Box
	for offset := start; offset < end; {
		if end-offset < 8 || len(boxes) >= 64 {
			return nil, errIFrameContainer
		}
		size := int(binary.BigEndian.Uint32(data[offset:]))
		if size < 8 || size > end-offset {
			return nil, errIFrameContainer
		}
		boxes = append(boxes, mp4Box{kind: string(data[offset+4 : offset+8]), start: offset, body: offset + 8, ends: offset + size})
		offset += size
	}
	return boxes, nil
}

func mp4Child(data []byte, parent mp4Box, path ...string) (mp4Box, error) {
	for _, kind := range path {
		children, err := mp4Boxes(data, parent.body, parent.ends)
		if err != nil {
			return mp4Box{}, err
		}
		found := false
		for _, child := range children {
			if child.kind == kind {
				parent, found = child, true
				break
			}
		}
		if !found {
			return mp4Box{}, errIFrameContainer
		}
	}
	return parent, nil
}

func splitIFrameFile(data []byte) (initialization, fragment []byte, timescale uint32, err error) {
	if len(data) > maximumIFrameFileBytes {
		return nil, nil, 0, errIFrameContainer
	}
	boxes, err := mp4Boxes(data, 0, len(data))
	if err != nil || len(boxes) < 4 || boxes[0].kind != "ftyp" {
		return nil, nil, 0, errIFrameContainer
	}
	split := -1
	var moov mp4Box
	for index, box := range boxes {
		if box.kind == "moov" && split < 0 {
			moov = box
		}
		if box.kind == "moof" && split < 0 {
			split = index
		}
	}
	if moov.kind == "" || split < 0 || split+1 >= len(boxes) || boxes[split+1].kind != "mdat" {
		return nil, nil, 0, errIFrameContainer
	}
	header, err := mp4Child(data, moov, "trak", "mdia", "mdhd")
	if err != nil || header.ends-header.body < 24 {
		return nil, nil, 0, errIFrameContainer
	}
	field := header.body + 12
	if data[header.body] == 1 {
		field = header.body + 20
	}
	if field+4 > header.ends {
		return nil, nil, 0, errIFrameContainer
	}
	timescale = binary.BigEndian.Uint32(data[field:])
	if timescale == 0 {
		return nil, nil, 0, errIFrameContainer
	}
	return data[:boxes[split].start], data[boxes[split].start:boxes[split+1].ends], timescale, nil
}

func retimeIFrameFragment(fragment []byte, decodeTime uint64, sequence uint32) ([]byte, error) {
	result := append([]byte(nil), fragment...)
	boxes, err := mp4Boxes(result, 0, len(result))
	if err != nil || len(boxes) != 2 || boxes[0].kind != "moof" || boxes[1].kind != "mdat" {
		return nil, errIFrameContainer
	}
	header, err := mp4Child(result, boxes[0], "mfhd")
	if err != nil || header.ends-header.body < 8 {
		return nil, errIFrameContainer
	}
	binary.BigEndian.PutUint32(result[header.body+4:], sequence)
	timing, err := mp4Child(result, boxes[0], "traf", "tfdt")
	if err != nil {
		return nil, err
	}
	switch {
	case result[timing.body] == 1 && timing.ends-timing.body >= 12:
		binary.BigEndian.PutUint64(result[timing.body+4:], decodeTime)
	case result[timing.body] == 0 && timing.ends-timing.body >= 8 && decodeTime <= 0xffffffff:
		binary.BigEndian.PutUint32(result[timing.body+4:], uint32(decodeTime))
	default:
		return nil, errIFrameContainer
	}
	return result, nil
}
