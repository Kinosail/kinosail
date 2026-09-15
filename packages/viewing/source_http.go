package viewing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func viewingSourceJSON(ctx context.Context, client *http.Client, input Input, path string, query url.Values, target any) error {
	return viewingSourceJSONHeaders(ctx, client, input, path, query, nil, target)
}

func viewingSourceJSONHeaders(ctx context.Context, client *http.Client, input Input, path string, query url.Values, headers map[string]string, target any) error {
	endpoint, err := url.Parse(input.URL)
	if err != nil {
		return errors.New("source request could not be created")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	endpoint.RawQuery = query.Encode()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Kinosail Viewing Activity Import")
	if input.Source == "plex" {
		request.Header.Set("X-Plex-Token", input.Token)
		request.Header.Set("X-Plex-Client-Identifier", "kinosail-viewing-import")
		request.Header.Set("X-Plex-Product", "Kinosail")
	} else {
		request.Header.Set("X-Emby-Token", input.Token)
		request.Header.Set("X-MediaBrowser-Token", input.Token)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%s source is unavailable", viewingSourceName(input.Source))
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s source returned %s", viewingSourceName(input.Source), response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximumViewingSourceBody+1))
	if err != nil {
		return fmt.Errorf("%s source response is too large", viewingSourceName(input.Source))
	}
	if len(data) > maximumViewingSourceBody {
		return fmt.Errorf("%s source response is too large", viewingSourceName(input.Source))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%s source returned invalid data", viewingSourceName(input.Source))
	}
	return nil
}

func viewingSourceName(source string) string {
	if source == "plex" {
		return "Plex"
	}
	return "Jellyfin"
}
