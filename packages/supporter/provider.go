package supporter

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

type activationRequest struct {
	Key             string  `json:"key"`
	AppID           string  `json:"appId"`
	InstallationKey string  `json:"installationKey"`
	ActivationID    string  `json:"activationId,omitempty"`
	RecognitionName *string `json:"recognitionName,omitempty"`
}

type activationResponse struct {
	Certificate  string `json:"certificate"`
	Signature    string `json:"signature"`
	PublicKey    string `json:"publicKey"`
	ActivationID string `json:"activationId"`
	Error        string `json:"error"`
}

func (service *Service) send(ctx context.Context, input activationRequest) (activationResponse, error) { //nolint:cyclop // Provider validation remains below the repository complexity ceiling.
	body, _ := json.Marshal(input)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, service.endpoint, strings.NewReader(string(body)))
	if err != nil {
		return activationResponse{}, invalid("could not prepare supporter activation")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))
	response, err := service.client.Do(request)
	if err != nil {
		return activationResponse{}, uncertain("supporter activation is unavailable; your local app is unchanged")
	}
	defer response.Body.Close()
	output, object, readErr := readActivationResponse(response)
	if readErr != nil {
		return activationResponse{}, readErr
	}
	if response.StatusCode != http.StatusOK {
		return activationResponse{}, service.providerError(response.StatusCode, object, output.Error)
	}
	if !exactFields(object, []string{"certificate", "signature", "publicKey", "activationId"}, nil) ||
		output.Certificate == "" || output.Signature == "" || output.PublicKey == "" || output.ActivationID == "" || output.Error != "" {
		return activationResponse{}, invalid("supporter activation returned an invalid response")
	}
	return output, nil
}

func readActivationResponse(response *http.Response) (activationResponse, map[string]json.RawMessage, error) {
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 8193))
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if readErr != nil || len(data) > 8192 || mediaErr != nil || mediaType != "application/json" {
		return activationResponse{}, nil, unavailable("supporter activation returned an invalid response")
	}
	object, objectErr := jsonObject(data)
	if objectErr != nil {
		return activationResponse{}, nil, unavailable("supporter activation returned an invalid response")
	}
	var output activationResponse
	if json.Unmarshal(data, &output) != nil {
		return activationResponse{}, nil, unavailable("supporter activation returned an invalid response")
	}
	return output, object, nil
}

func (service *Service) providerError(status int, object map[string]json.RawMessage, code string) error {
	if !exactFields(object, []string{"error"}, nil) || status != http.StatusBadRequest {
		return unavailable("supporter activation is unavailable; your local app is unchanged")
	}
	switch code {
	case "supporter_key_not_accepted", "supporter key was not accepted":
		return invalid("supporter key was not accepted")
	case "supporter_key_expired", "supporter key has expired":
		return invalid("supporter key has expired")
	case "invalid_request":
		if service.app.AcceptInvalidRequestError {
			return invalid("supporter key was not accepted")
		}
		fallthrough
	default:
		return unavailable("supporter activation is unavailable; your local app is unchanged")
	}
}
