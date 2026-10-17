package jellyfincompat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"sync"
)

const streamBufferBytes = 32 << 10

var streamBuffers sync.Pool

type Page struct {
	Items            any `json:"Items"`
	StartIndex       int `json:"StartIndex"`
	TotalRecordCount int `json:"TotalRecordCount"`
}

type Records struct {
	entries []catalogEntry
	project ProjectItem
}

type jsonStream interface {
	writeJSON(io.Writer) error
}

type streamEncoder struct {
	writer  io.Writer
	buffer  bytes.Buffer
	encoder *json.Encoder
	err     error
}

func Result[T any](items []T, start int, totals ...int) Page {
	total := len(items)
	if len(totals) > 0 {
		total = totals[0]
	}
	return Page{Items: items, StartIndex: start, TotalRecordCount: total}
}

func (records Records) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	err := records.writeJSON(&buffer)
	return buffer.Bytes(), err
}

func (records Records) writeJSON(writer io.Writer) error {
	out := newStreamEncoder(writer)
	if records.entries == nil {
		out.raw("null")
		return out.err
	}
	var item ItemRecord
	var series SeriesRecord
	var season SeasonRecord
	out.raw("[")
	for position, entry := range records.entries {
		if position > 0 {
			out.raw(",")
		}
		switch {
		case entry.item != nil:
			item = records.project(*entry.item)
			out.value(&item)
		case entry.kind == "Season":
			season = SeasonDTO(*entry.show, entry.season)
			out.value(&season)
		default:
			series = SeriesDTO(*entry.show)
			out.value(&series)
		}
		if out.err != nil {
			return out.err
		}
	}
	out.raw("]")
	return out.err
}

func (page Page) writeJSON(writer io.Writer) error {
	out := newStreamEncoder(writer)
	out.raw(`{"Items":`)
	if records, streamed := page.Items.(Records); streamed {
		if out.err == nil {
			out.err = records.writeJSON(writer)
		}
	} else {
		out.value(page.Items)
	}
	out.raw(`,"StartIndex":` + strconv.Itoa(page.StartIndex) + `,"TotalRecordCount":` + strconv.Itoa(page.TotalRecordCount) + "}")
	return out.err
}

func newStreamEncoder(writer io.Writer) *streamEncoder {
	out := &streamEncoder{writer: writer}
	out.encoder = json.NewEncoder(&out.buffer)
	return out
}

func (out *streamEncoder) raw(text string) {
	if out.err == nil {
		_, out.err = io.WriteString(out.writer, text)
	}
}

func (out *streamEncoder) value(value any) {
	if out.err != nil {
		return
	}
	out.buffer.Reset()
	if out.err = out.encoder.Encode(value); out.err == nil {
		_, out.err = out.writer.Write(bytes.TrimSuffix(out.buffer.Bytes(), []byte("\n")))
	}
}

func writeJSONStream(writer io.Writer, stream jsonStream) {
	buffered, pooled := streamBuffers.Get().(*bufio.Writer)
	if !pooled {
		buffered = bufio.NewWriterSize(nil, streamBufferBytes)
	}
	buffered.Reset(writer)
	if stream.writeJSON(buffered) == nil && buffered.WriteByte('\n') == nil {
		_ = buffered.Flush()
	}
	buffered.Reset(nil)
	streamBuffers.Put(buffered)
}
