package playback

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maximumJellyfinBitrateTest = 10_000_000

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}

func JellyfinBitrateTest(writer http.ResponseWriter, request *http.Request) {
	size, valid := JellyfinBitrateTestSize(request)
	if !valid {
		http.Error(writer, "invalid bitrate test size", http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Set("Content-Length", strconv.Itoa(size))
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = io.CopyN(writer, zeroReader{}, int64(size))
}

func JellyfinBitrateTestSize(request *http.Request) (int, bool) { //nolint:cyclop // Alias and cardinality validation form one strict query parser.
	if request == nil || request.URL == nil {
		return 0, false
	}
	size, found := 102400, false
	for key, values := range request.URL.Query() {
		if found || !strings.EqualFold(key, "size") || len(values) != 1 || len(values[0]) > 8 {
			return 0, false
		}
		value, err := strconv.Atoi(values[0])
		if err != nil || value < 1 || value > maximumJellyfinBitrateTest {
			return 0, false
		}
		size, found = value, true
	}
	return size, true
}
