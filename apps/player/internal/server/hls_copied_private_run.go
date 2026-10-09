package server

import (
	"context"
	"encoding/binary"
	"math"
)

type copiedHLSPrivateDefaults struct {
	id, duration, size, flags uint32
}

func copiedHLSPrivateFragmentDefaults(data []byte) (copiedHLSPrivateDefaults, error) {
	var defaults copiedHLSPrivateDefaults
	if len(data) < 8 || data[0] != 0 {
		return defaults, errCopiedHLSIndex
	}
	flags := binary.BigEndian.Uint32(data[:4]) & 0xffffff
	if flags&0x020038 != 0x020038 || flags&^uint32(0x02003a) != 0 {
		return defaults, errCopiedHLSIndex
	}
	defaults.id = binary.BigEndian.Uint32(data[4:8])
	position, err := copiedHLSPrivateDescription(data, flags)
	if err != nil {
		return defaults, err
	}
	values := [3]uint32{}
	for index := range values {
		value, err := copiedHLSPrivateUint32(data, &position)
		if err != nil {
			return defaults, err
		}
		values[index] = value
	}
	defaults.duration, defaults.size, defaults.flags = values[0], values[1], values[2]
	if !defaults.complete(position, uint64(len(data))) {
		return defaults, errCopiedHLSIndex
	}
	return defaults, nil
}

func copiedHLSPrivateDescription(data []byte, flags uint32) (uint64, error) {
	position := uint64(8)
	if flags&2 == 0 {
		return position, nil
	}
	description, err := copiedHLSPrivateUint32(data, &position)
	if err != nil || description != 1 {
		return 0, errCopiedHLSIndex
	}
	return position, nil
}

func (defaults copiedHLSPrivateDefaults) complete(position, length uint64) bool {
	return position == length && defaults.id != 0 && defaults.duration != 0 && defaults.size != 0
}

type copiedHLSPrivateRun struct {
	span     copiedHLSPrivateSpan
	flags    uint32
	count    uint32
	position uint64
}

func copiedHLSPrivateSampleRun(ctx context.Context, data []byte, defaults copiedHLSPrivateDefaults, base uint64, media copiedHLSPrivateBox) (copiedHLSPrivateSpan, error) {
	run, err := copiedHLSPrivateStartRun(data, defaults, base)
	if err != nil {
		return run.span, err
	}
	for index := uint32(0); index < run.count; index++ {
		size, err := copiedHLSPrivateSampleSize(data, run.flags, defaults, &run.position)
		if err != nil || ctx.Err() != nil || !copiedHLSPrivateSampleExtent(run.span.end, size, media) {
			return run.span, errCopiedHLSIndex
		}
		run.span.end += uint64(size)
		if index == 0 {
			run.span.firstEnd = run.span.end
		}
	}
	if run.position != uint64(len(data)) {
		return run.span, errCopiedHLSIndex
	}
	return run.span, nil
}

func copiedHLSPrivateStartRun(data []byte, defaults copiedHLSPrivateDefaults, base uint64) (copiedHLSPrivateRun, error) {
	var run copiedHLSPrivateRun
	var err error
	run.flags, run.count, run.position, err = copiedHLSPrivateRunHeader(data)
	if err != nil {
		return run, err
	}
	offset, err := copiedHLSPrivateUint32(data, &run.position)
	if err != nil || offset == 0 || offset > math.MaxInt32 {
		return run, errCopiedHLSIndex
	}
	run.span.id, run.span.start, run.span.end = defaults.id, base+uint64(offset), base+uint64(offset)
	if run.flags&4 != 0 {
		if _, err := copiedHLSPrivateUint32(data, &run.position); err != nil {
			return run, err
		}
	}
	return run, nil
}

func copiedHLSPrivateRunHeader(data []byte) (uint32, uint32, uint64, error) {
	if len(data) < 12 || data[0] > 1 {
		return 0, 0, 0, errCopiedHLSIndex
	}
	flags := binary.BigEndian.Uint32(data[:4]) & 0xffffff
	count := binary.BigEndian.Uint32(data[4:8])
	if flags&^uint32(0xf05) != 0 || flags&1 == 0 || flags&4 != 0 && flags&0x400 != 0 ||
		count == 0 || count > 4096 {
		return 0, 0, 0, errCopiedHLSIndex
	}
	return flags, count, 8, nil
}

func copiedHLSPrivateSampleSize(data []byte, flags uint32, defaults copiedHLSPrivateDefaults, position *uint64) (uint32, error) {
	values := [4]uint32{defaults.duration, defaults.size, defaults.flags, 0}
	for index := range values {
		mask := uint32(0x100) << index
		if flags&mask != 0 {
			value, err := copiedHLSPrivateUint32(data, position)
			if err != nil {
				return 0, err
			}
			values[index] = value
		}
	}
	if values[0] == 0 || values[1] == 0 {
		return 0, errCopiedHLSIndex
	}
	return values[1], nil
}

func copiedHLSPrivateSampleExtent(start uint64, size uint32, media copiedHLSPrivateBox) bool {
	return start >= media.start+media.header && start < media.end && uint64(size) <= media.end-start
}
