package server

import (
	"encoding/binary"
	"slices"
)

type copiedHLSPrivateBox struct {
	kind               string
	data               []byte
	start, end, header uint64
}

func copiedHLSPrivateBoxes(data []byte, allowed ...string) ([]copiedHLSPrivateBox, error) {
	var boxes []copiedHLSPrivateBox
	for position := uint64(0); position < uint64(len(data)); {
		if len(boxes) >= 64 || uint64(len(data))-position < 8 {
			return nil, errCopiedHLSIndex
		}
		box, err := copiedHLSPrivateNextBox(data, position)
		if err != nil || !slices.Contains(allowed, box.kind) {
			return nil, errCopiedHLSIndex
		}
		boxes = append(boxes, box)
		position = box.end
	}
	return boxes, nil
}

func copiedHLSPrivateNextBox(data []byte, start uint64) (copiedHLSPrivateBox, error) {
	if start > uint64(len(data)) || uint64(len(data))-start < 8 {
		return copiedHLSPrivateBox{}, errCopiedHLSIndex
	}
	box := copiedHLSPrivateBox{start: start, header: 8, kind: string(data[start+4 : start+8])}
	size := uint64(binary.BigEndian.Uint32(data[start : start+4]))
	if size == 1 {
		if uint64(len(data))-start < 16 {
			return box, errCopiedHLSIndex
		}
		box.header, size = 16, binary.BigEndian.Uint64(data[start+8:start+16])
	}
	if size == 0 {
		size = uint64(len(data)) - start
	}
	if size < box.header || size > uint64(len(data))-start {
		return box, errCopiedHLSIndex
	}
	box.end = start + size
	box.data = data[start+box.header : box.end]
	return box, nil
}

func copiedHLSPrivateOne(boxes []copiedHLSPrivateBox, kind string) (copiedHLSPrivateBox, error) {
	var selected copiedHLSPrivateBox
	for _, box := range boxes {
		if box.kind == kind {
			if selected.kind != "" {
				return selected, errCopiedHLSIndex
			}
			selected = box
		}
	}
	if selected.kind == "" {
		return selected, errCopiedHLSIndex
	}
	return selected, nil
}

func copiedHLSPrivateUint32(data []byte, position *uint64) (uint32, error) {
	if *position > uint64(len(data)) || uint64(len(data))-*position < 4 {
		return 0, errCopiedHLSIndex
	}
	value := binary.BigEndian.Uint32(data[*position : *position+4])
	*position += 4
	return value, nil
}
