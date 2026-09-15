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

func (provider *subtitleProvider) json(ctx context.Context, endpoint string, target any) error {
	if err := provider.health.before("SubDL"); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Kinosail Subtitles/0.1")
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.client.Do(request)
	if err != nil {
		provider.health.observe("SubDL", nil, err)
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		provider.health.observe("SubDL", response, errors.New("request failed"))
		return errors.New("subtitle provider rejected request")
	}
	err = decodeExternalJSON(response.Body, 1<<20, target)
	provider.health.observe("SubDL", response, err)
	return err
}

func (provider *subtitleProvider) download(ctx context.Context, link string) ([]byte, error) {
	if err := provider.health.before("SubDL"); err != nil {
		return nil, err
	}
	link = provider.resolve(link)
	if !provider.allowed(link) {
		return nil, errors.New("subtitle download link is not trusted")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	response, err := provider.client.Do(request)
	if err != nil {
		provider.health.observe("SubDL", nil, err)
		return nil, err
	}
	defer response.Body.Close()
	data, err := readSubtitle(response)
	if err != nil {
		provider.health.observe("SubDL", response, err)
		return nil, err
	}
	if archive, zipErr := zip.NewReader(bytes.NewReader(data), int64(len(data))); zipErr == nil {
		data, err = readArchivedSubtitle(archive)
		if err != nil {
			provider.health.observe("SubDL", response, err)
			return nil, err
		}
	} else if !validSubtitlePayload(data) {
		err = errors.New("subtitle download is invalid")
		provider.health.observe("SubDL", response, err)
		return nil, err
	}
	provider.health.observe("SubDL", response, nil)
	return data, nil
}

func readSubtitle(response *http.Response) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 || response.StatusCode != http.StatusOK {
		return nil, errors.New("subtitle download is invalid")
	}
	return data, nil
}

func readArchivedSubtitle(archive *zip.Reader) ([]byte, error) { //nolint:cyclop // Archive entries are bounded, filtered, read, and ambiguity-checked in one operation.
	var subtitle []byte
	for _, file := range archive.File {
		extension := strings.ToLower(filepath.Ext(file.Name))
		if file.FileInfo().IsDir() || !oneOf(extension, ".srt", ".vtt") || file.UncompressedSize64 > 4<<20 {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, (4<<20)+1))
		_ = reader.Close()
		if readErr == nil && len(data) <= 4<<20 && validSubtitlePayload(data) {
			if subtitle != nil {
				return nil, errors.New("subtitle archive is ambiguous")
			}
			subtitle = data
		}
	}
	if subtitle != nil {
		return subtitle, nil
	}
	return nil, errors.New("subtitle archive is invalid")
}

func validSubtitlePayload(data []byte) bool {
	decoded, err := decodeSubtitleText(data)
	return err == nil && bytes.Contains(decoded, []byte("-->"))
}

func (provider *subtitleProvider) allowed(link string) bool {
	base, baseErr := url.Parse(provider.config.URL)
	download, downloadErr := url.Parse(link)
	if baseErr != nil || downloadErr != nil || download.Host == "" {
		return false
	}
	if download.Host == base.Host {
		return download.Scheme == base.Scheme
	}
	return download.Scheme == "https" && strings.HasSuffix(strings.ToLower(download.Hostname()), ".subdl.com")
}

func (provider *subtitleProvider) resolve(link string) string {
	download, err := url.Parse(link)
	if err != nil || download.IsAbs() {
		return link
	}
	base, err := url.Parse(provider.config.URL)
	if err != nil {
		return link
	}
	base.Path, base.RawQuery = "/", ""
	if strings.HasSuffix(strings.ToLower(base.Hostname()), "subdl.com") {
		base.Scheme, base.Host = "https", "dl.subdl.com"
	}
	return base.ResolveReference(download).String()
}
