package federation

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

func fetchSAMLMetadata(ctx context.Context, address string) (*saml.EntityDescriptor, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	response, err := samlHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("SAML metadata is unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxSAMLMetadata+1))
	if err != nil || len(data) > maxSAMLMetadata {
		return nil, errors.New("SAML metadata is too large or unreadable")
	}
	return samlsp.ParseMetadata(data)
}
