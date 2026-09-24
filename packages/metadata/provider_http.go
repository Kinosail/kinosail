package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

// GetProviderJSON reads one bounded response using the configured provider client.
func GetProviderJSON(ctx context.Context, client *http.Client, endpoint, token string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("metadata provider rejected request")
	}
	return decodeExternalJSON(response.Body, 2<<20, target)
}

// DownloadProviderImage validates a bounded image response before replacing its cache file.
func DownloadProviderImage(ctx context.Context, client *http.Client, baseURL, path, target string) error {
	endpoint := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(data) > 8<<20 || response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "image/") {
		return errors.New("metadata artwork is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return privatefile.WriteCache(target, data)
}
