package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	subtitleSpeechMagic      = "KSP1"
	subtitleSpeechCacheLimit = 1024
	subtitleSpeechFileLimit  = 4 << 20
)

func quantizeSubtitleSpeech(reference subtitleAudioReference) subtitleAudioReference {
	quantize := func(values []float64) []float64 {
		output := make([]float64, len(values))
		for position, value := range values {
			output[position] = math.Round(max(0, min(1, value))*255) / 255
		}
		return output
	}
	return subtitleAudioReference{Speech: quantize(reference.Speech), Waveform: quantize(reference.Waveform)}
}

func (synchronizer *subtitleSynchronizer) speechPath(key string) string {
	if synchronizer.cacheDir == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(key))
	return filepath.Join(synchronizer.cacheDir, "subtitle-speech", hex.EncodeToString(digest[:])+".bin")
}

func (synchronizer *subtitleSynchronizer) loadSpeech(key string) (subtitleAudioReference, bool) {
	path := synchronizer.speechPath(key)
	if path == "" {
		return subtitleAudioReference{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return subtitleAudioReference{}, false
	}
	defer file.Close()
	reader, err := gzip.NewReader(io.LimitReader(file, subtitleSpeechFileLimit))
	if err != nil {
		return subtitleAudioReference{}, false
	}
	maximumFrames := int(subtitleAudioLimit / subtitleFrame)
	data, err := io.ReadAll(io.LimitReader(reader, int64(12+maximumFrames+maximumFrames/subtitleWaveformFrames+2)))
	if err != nil || len(data) < 12 || string(data[:4]) != subtitleSpeechMagic {
		return subtitleAudioReference{}, false
	}
	speech, waveform := int(binary.LittleEndian.Uint32(data[4:])), int(binary.LittleEndian.Uint32(data[8:]))
	if speech < int(time.Minute/subtitleFrame) || speech > maximumFrames || waveform != (speech+subtitleWaveformFrames-1)/subtitleWaveformFrames || len(data) != 12+speech+waveform {
		return subtitleAudioReference{}, false
	}
	dequantize := func(values []byte) []float64 {
		output := make([]float64, len(values))
		for position, value := range values {
			output[position] = float64(value) / 255
		}
		return output
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	return subtitleAudioReference{Speech: dequantize(data[12 : 12+speech]), Waveform: dequantize(data[12+speech:])}, true
}

func (synchronizer *subtitleSynchronizer) saveSpeech(key string, reference subtitleAudioReference) {
	path := synchronizer.speechPath(key)
	if path == "" {
		return
	}
	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	header := make([]byte, 12)
	copy(header, subtitleSpeechMagic)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(reference.Speech)))
	binary.LittleEndian.PutUint32(header[8:], uint32(len(reference.Waveform)))
	values := make([]byte, 0, len(reference.Speech)+len(reference.Waveform))
	for _, value := range append(append([]float64(nil), reference.Speech...), reference.Waveform...) {
		values = append(values, byte(math.Round(max(0, min(1, value))*255)))
	}
	_, _ = writer.Write(header)
	_, _ = writer.Write(values)
	if writer.Close() != nil || writeAtomicFile(path, encoded.Bytes()) != nil {
		slog.Warn("speech analysis cache write failed", "error_class", "filesystem")
		return
	}
	pruneSubtitleSpeechCache(filepath.Dir(path))
}

func pruneSubtitleSpeechCache(directory string) {
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) <= subtitleSpeechCacheLimit {
		return
	}
	type cached struct {
		name     string
		modified time.Time
	}
	files := make([]cached, 0, len(entries))
	for _, entry := range entries {
		if info, infoErr := entry.Info(); infoErr == nil && info.Mode().IsRegular() && filepath.Ext(entry.Name()) == ".bin" {
			files = append(files, cached{entry.Name(), info.ModTime()})
		}
	}
	sort.Slice(files, func(left, right int) bool { return files[left].modified.Before(files[right].modified) })
	for _, file := range files[:max(0, len(files)-subtitleSpeechCacheLimit)] {
		_ = os.Remove(filepath.Join(directory, file.name))
	}
}
