package httpcompress

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
)

type mode int

const (
	undecided mode = iota
	passthrough
	buffering
	compressing
)

type responseWriter struct {
	http.ResponseWriter
	allowed bool
	status  int
	mode    mode
	buffer  []byte
	encoder *gzip.Writer
}

func (writer *responseWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *responseWriter) WriteHeader(status int) {
	if writer.mode != undecided {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		writer.ResponseWriter.WriteHeader(status)
		return
	}
	writer.status = status
	writer.mode = writer.decide(status)
	switch writer.mode {
	case passthrough:
		writer.ResponseWriter.WriteHeader(status)
	case compressing:
		writer.start()
	case undecided, buffering:
	}
}

func (writer *responseWriter) decide(status int) mode {
	header := writer.Header()
	if !compressible(header.Get("Content-Type")) {
		return passthrough
	}
	AddVary(header)
	if !writer.allowed || status != http.StatusOK || header.Get("Content-Encoding") != "" || header.Get("Content-Range") != "" {
		return passthrough
	}
	if declared := header.Get("Content-Length"); declared != "" {
		if length, err := strconv.ParseInt(declared, 10, 64); err != nil || length < MinimumSize {
			return passthrough
		}
		return compressing
	}
	return buffering
}

func (writer *responseWriter) start() {
	header := writer.Header()
	header.Del("Content-Length")
	header.Set("Content-Encoding", "gzip")
	weakenETag(header)
	writer.ResponseWriter.WriteHeader(writer.status)
	writer.encoder = writers.Get().(*gzip.Writer)
	writer.encoder.Reset(writer.ResponseWriter)
	writer.mode = compressing
}

func (writer *responseWriter) Write(data []byte) (int, error) {
	if writer.mode == undecided {
		writer.WriteHeader(http.StatusOK)
	}
	switch writer.mode {
	case buffering:
		writer.buffer = append(writer.buffer, data...)
		if len(writer.buffer) < MinimumSize {
			return len(data), nil
		}
		writer.start()
		pending := writer.buffer
		writer.buffer = nil
		if _, err := writer.encoder.Write(pending); err != nil {
			return 0, err
		}
		return len(data), nil
	case compressing:
		return writer.encoder.Write(data)
	case undecided, passthrough:
	}
	return writer.ResponseWriter.Write(data)
}

func (writer *responseWriter) ReadFrom(source io.Reader) (int64, error) {
	if writer.mode == undecided {
		writer.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := writer.ResponseWriter.(io.ReaderFrom); ok && writer.mode == passthrough {
		return readerFrom.ReadFrom(source)
	}
	return io.Copy(struct{ io.Writer }{writer}, source)
}

func (writer *responseWriter) FlushError() error {
	if writer.mode == undecided {
		writer.WriteHeader(http.StatusOK)
	}
	if writer.mode == buffering {
		pending := writer.buffer
		writer.buffer = nil
		writer.start()
		if _, err := writer.encoder.Write(pending); err != nil {
			return err
		}
	}
	if writer.mode == compressing {
		if err := writer.encoder.Flush(); err != nil {
			return err
		}
	}
	return http.NewResponseController(writer.ResponseWriter).Flush()
}

func (writer *responseWriter) Flush() { _ = writer.FlushError() }

func (writer *responseWriter) finish() {
	switch writer.mode {
	case buffering:
		writer.Header().Set("Content-Length", strconv.Itoa(len(writer.buffer)))
		writer.ResponseWriter.WriteHeader(writer.status)
		_, _ = writer.ResponseWriter.Write(writer.buffer)
	case compressing:
		_ = writer.encoder.Close()
		writer.encoder.Reset(io.Discard)
		writers.Put(writer.encoder)
		writer.encoder = nil
	case undecided, passthrough:
	}
}
