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
	found := map[bool][][]byte{}
	for _, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		styled, wanted := archivedSubtitleKind(file)
		if !wanted {
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
		if readErr == nil && validArchivedSubtitle(data, styled) {
			found[styled] = append(found[styled], data)
		}
	}
	return archivedSubtitleChoice(found[false], found[true])
}

func archivedSubtitleKind(file *zip.File) (styled, wanted bool) {
	extension := strings.ToLower(filepath.Ext(file.Name))
	styled = oneOf(extension, ".ass", ".ssa")
	return styled, !file.FileInfo().IsDir() && (styled || oneOf(extension, ".srt", ".vtt")) && file.UncompressedSize64 <= 4<<20
}

func validArchivedSubtitle(data []byte, styled bool) bool {
	if len(data) > 4<<20 {
		return false
	}
	if styled {
		return validStyledSubtitlePayload(data)
	}
	return validSubtitlePayload(data)
}

func archivedSubtitleChoice(text, styled [][]byte) ([]byte, error) {
	if len(text) == 0 {
		text = styled
	}
	switch len(text) {
	case 0:
		return nil, errors.New("subtitle archive is invalid")
	case 1:
		return text[0], nil
	}
	return nil, errors.New("subtitle archive is ambiguous")
}

func validStyledSubtitlePayload(data []byte) bool {
	decoded, err := decodeSubtitleText(data)
	return err == nil && bytes.HasPrefix(bytes.TrimSpace(decoded), []byte("[Script Info]")) && bytes.Contains(decoded, []byte("Dialogue:"))
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
