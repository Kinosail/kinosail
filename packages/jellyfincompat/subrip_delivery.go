package jellyfincompat

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/MikeO7/kinosail/packages/playback"
)

type subRipCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (capture *subRipCapture) Header() http.Header { return capture.header }

func (capture *subRipCapture) WriteHeader(status int) {
	if capture.status == 0 {
		capture.status = status
	}
}

func (capture *subRipCapture) Write(data []byte) (int, error) {
	if capture.status == 0 {
		capture.status = http.StatusOK
	}
	if capture.body.Len()+len(data) > 16<<20 {
		return 0, http.ErrContentLength
	}
	return capture.body.Write(data)
}

func writeSubRip(writer http.ResponseWriter, request *http.Request, serve func(http.ResponseWriter, *http.Request)) {
	captured := request.Clone(request.Context())
	for _, name := range []string{"Accept-Encoding", "Range", "If-Range", "If-Modified-Since", "If-None-Match"} {
		captured.Header.Del(name)
	}
	capture := &subRipCapture{header: http.Header{}}
	serve(capture, captured)
	if capture.status != http.StatusOK {
		for name, values := range capture.header {
			writer.Header()[name] = values
		}
		writer.WriteHeader(max(capture.status, http.StatusOK))
		_, _ = writer.Write(capture.body.Bytes())
		return
	}
	data := playback.WebVTTToSubRip(capture.body.Bytes())
	writer.Header().Set("Content-Type", "application/x-subrip; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = writer.Write(data)
}
