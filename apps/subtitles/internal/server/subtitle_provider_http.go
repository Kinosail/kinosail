package server

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

var errSubtitleDownloadTooLarge = errors.New("subtitle download is invalid")

func subtitleProviderRedirect(allowed func(string) bool) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		endpoint := request.URL.String()
		if len(via) >= 10 || len(endpoint) > 4096 || request.URL.User != nil {
			return http.ErrUseLastResponse
		}
		if _, err := url.QueryUnescape(request.URL.RawQuery); err != nil {
			return http.ErrUseLastResponse
		}
		if !allowed(endpoint) {
			return http.ErrUseLastResponse
		}
		return nil
	}
}

func readSubtitle(response *http.Response) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || response.StatusCode != http.StatusOK {
		return nil, errors.New("subtitle download is invalid")
	}
	if len(data) > 4<<20 {
		return nil, errSubtitleDownloadTooLarge
	}
	return data, nil
}

func readArchivedSubtitle(ctx context.Context, archive *zip.Reader) ([]byte, error) {
	if len(archive.File) > 128 {
		return nil, errors.New("subtitle archive has too many entries")
	}
	remaining := int64(16 << 20)
	var subtitle []byte
	for _, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		extension := strings.ToLower(filepath.Ext(file.Name))
		if file.FileInfo().IsDir() || !oneOf(extension, ".srt", ".vtt") || file.UncompressedSize64 > 4<<20 {
			continue
		}
		if int64(file.UncompressedSize64) > remaining {
			return nil, errors.New("subtitle archive exceeds its decompression budget")
		}
		reader, err := file.Open()
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(subtitleContextReader{ctx, reader}, min(remaining, 4<<20)+1))
		_ = reader.Close()
		remaining -= int64(len(data))
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if remaining < 0 {
			return nil, errors.New("subtitle archive exceeds its decompression budget")
		}
		if readErr != nil || len(data) > 4<<20 || !validSubtitlePayload(data) {
			continue
		}
		if subtitle != nil {
			return nil, errors.New("subtitle archive is ambiguous")
		}
		subtitle = data
	}
	if subtitle != nil {
		return subtitle, nil
	}
	return nil, errors.New("subtitle archive is invalid")
}

type subtitleContextReader struct {
	context.Context
	io.Reader
}

func (reader subtitleContextReader) Read(buffer []byte) (int, error) {
	if err := reader.Err(); err != nil {
		return 0, err
	}
	return reader.Reader.Read(buffer[:min(len(buffer), 32<<10)])
}

func validSubtitlePayload(data []byte) bool {
	if decoded, err := decodeSubtitleText(data); err == nil {
		return bytes.Contains(decoded, []byte("-->"))
	}
	return len(data) > 0 && len(data) <= 4<<20 && !bytes.ContainsRune(data, 0) && bytes.Contains(data, []byte("-->"))
}
