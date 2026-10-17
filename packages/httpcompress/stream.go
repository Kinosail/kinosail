package httpcompress

import (
	"compress/gzip"
	"net/http"
	"strconv"
)

type Stream struct {
	writer  http.ResponseWriter
	status  int
	gzip    bool
	started bool
	pending []byte
	encoder *gzip.Writer
}

func NewStream(writer http.ResponseWriter, request *http.Request, status int) *Stream {
	AddVary(writer.Header())
	return &Stream{writer: writer, status: status, gzip: GzipRequested(request)}
}

func (stream *Stream) Write(data []byte) (int, error) {
	if !stream.gzip {
		stream.start()
		return stream.writer.Write(data)
	}
	if stream.encoder == nil {
		if len(stream.pending)+len(data) < MinimumSize {
			stream.pending = append(stream.pending, data...)
			return len(data), nil
		}
		if err := stream.compress(); err != nil {
			return 0, err
		}
	}
	return stream.encoder.Write(data)
}

func (stream *Stream) Close() error {
	if stream.encoder != nil {
		err := stream.encoder.Close()
		stream.encoder.Reset(nil)
		writers.Put(stream.encoder)
		stream.encoder = nil
		return err
	}
	if !stream.started {
		stream.writer.Header().Set("Content-Length", strconv.Itoa(len(stream.pending)))
	}
	stream.start()
	if len(stream.pending) == 0 {
		return nil
	}
	_, err := stream.writer.Write(stream.pending)
	stream.pending = nil
	return err
}

func (stream *Stream) start() {
	if !stream.started {
		stream.started = true
		stream.writer.WriteHeader(stream.status)
	}
}

func (stream *Stream) compress() error {
	header := stream.writer.Header()
	header.Del("Content-Length")
	header.Set("Content-Encoding", "gzip")
	stream.start()
	stream.encoder, _ = writers.Get().(*gzip.Writer)
	stream.encoder.Reset(stream.writer)
	pending := stream.pending
	stream.pending = nil
	_, err := stream.encoder.Write(pending)
	return err
}
