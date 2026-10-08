package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"strconv"
	"strings"
)

type copiedHLSNormalizedAudioRow struct {
	pts, samples int64
}

type copiedHLSNormalizedAudioScan struct {
	ctx     context.Context
	clock   *copiedHLSSourceAudioClock
	rows    []copiedHLSNormalizedAudioRow
	headers map[string]int
}

func readCopiedHLSNormalizedAudio(ctx context.Context, data []byte, clock *copiedHLSSourceAudioClock) ([]copiedHLSNormalizedAudioRow, error) {
	if len(data) == 0 || len(data) > 2<<20 {
		return nil, errCopiedHLSIndex
	}
	scan := copiedHLSNormalizedAudioScan{ctx: ctx, clock: clock, headers: make(map[string]int)}
	err := scanCopiedHLSLines(bytes.NewReader(data), 2<<20, 1040, scan.visit)
	if err != nil || ctx.Err() != nil || len(scan.rows) != 1024 || !completeCopiedHLSNormalizedAudioHeaders(scan.headers) {
		return nil, errCopiedHLSIndex
	}
	return scan.rows, nil
}

func (scan *copiedHLSNormalizedAudioScan) visit(line string) error {
	if scan.ctx.Err() != nil {
		return errCopiedHLSIndex
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if strings.HasPrefix(line, "#") {
		if !copiedHLSNormalizedAudioHeader(line, scan.headers) {
			return errCopiedHLSIndex
		}
		return nil
	}
	row, ok := parseCopiedHLSNormalizedAudioRow(line)
	if !ok || len(scan.rows) >= 1024 || !matchesCopiedHLSNormalizedAudio(row, scan.clock, len(scan.rows)) {
		return errCopiedHLSIndex
	}
	scan.rows = append(scan.rows, row)
	return nil
}

func completeCopiedHLSNormalizedAudioHeaders(headers map[string]int) bool {
	return headers["#tb "] == 1 && headers["#media_type "] == 1 && headers["#sample_rate "] == 1
}

func copiedHLSNormalizedAudioHeader(line string, headers map[string]int) bool {
	expected := map[string]string{
		"#tb ":          "#tb 0: 1/48000",
		"#media_type ":  "#media_type 0: audio",
		"#sample_rate ": "#sample_rate 0: 48000",
	}
	for prefix, canonical := range expected {
		if strings.HasPrefix(line, prefix) {
			headers[prefix]++
			return line == canonical && headers[prefix] == 1
		}
	}
	return true
}

func parseCopiedHLSNormalizedAudioRow(line string) (copiedHLSNormalizedAudioRow, bool) {
	var row copiedHLSNormalizedAudioRow
	fields := strings.Split(line, ",")
	if len(fields) != 6 {
		return row, false
	}
	var values [5]int64
	for number := range values {
		value, err := strconv.ParseInt(strings.TrimSpace(fields[number]), 10, 64)
		if err != nil {
			return row, false
		}
		values[number] = value
	}
	if !validCopiedHLSNormalizedAudioHash(strings.TrimSpace(fields[5])) {
		return row, false
	}
	row.pts, row.samples = values[2], values[3]
	return row, validCopiedHLSNormalizedAudioValues(values)
}

func validCopiedHLSNormalizedAudioHash(digest string) bool {
	hash, err := hex.DecodeString(digest)
	return len(digest) == 32 && err == nil && hex.EncodeToString(hash) == digest
}

func validCopiedHLSNormalizedAudioValues(values [5]int64) bool {
	return values[0] == 0 && values[1] == values[2] && values[2] >= 0 && values[2] <= 22*48000 &&
		values[3] > 0 && values[3] <= 1024 && values[4] == values[3]*4
}

func matchesCopiedHLSNormalizedAudio(row copiedHLSNormalizedAudioRow, clock *copiedHLSSourceAudioClock, number int) bool {
	frame := clock.frames[number]
	return row.samples == *frame.Samples && row.pts == frame.native+clock.samplePhase(number)
}
