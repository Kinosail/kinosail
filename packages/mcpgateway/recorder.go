package mcpgateway

import (
	"bytes"
	"net/http"
)

type mcpAPIRecorder struct {
	header http.Header
	body   mcpBoundedBuffer
	status int
}

func (writer *mcpAPIRecorder) Header() http.Header { return writer.header }
func (writer *mcpAPIRecorder) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *mcpAPIRecorder) Write(data []byte) (int, error) {
	writer.WriteHeader(http.StatusOK)
	return writer.body.Write(data)
}

func (writer *mcpAPIRecorder) statusCode() int {
	if writer.status == 0 {
		return http.StatusOK
	}
	return writer.status
}

type mcpBoundedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (buffer *mcpBoundedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := buffer.limit - buffer.Len()
	if remaining < len(data) {
		buffer.exceeded = true
		data = data[:max(0, remaining)]
	}
	_, _ = buffer.Buffer.Write(data)
	return written, nil
}
