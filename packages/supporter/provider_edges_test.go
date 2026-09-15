package supporter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReadCloser) Close() error             { return nil }

func TestSendRejectsMalformedInternalEndpointBeforeNetwork(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	service.endpoint = ":"
	if _, err := service.send(context.Background(), activationRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("send malformed endpoint error = %v", err)
	}
}

func TestReadActivationResponseRejectsReadAndDecodeFailures(t *testing.T) {
	readFailure := &http.Response{Header: http.Header{"Content-Type": []string{"application/json"}}, Body: failingReadCloser{}}
	if _, _, err := readActivationResponse(readFailure); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("read failure error = %v", err)
	}
	typeFailure := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"certificate":1,"signature":"x","publicKey":"x","activationId":"x"}`)),
	}
	if _, _, err := readActivationResponse(typeFailure); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("decode failure error = %v", err)
	}
}

func TestProviderReportsRejectedKeyPrecisely(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	object := map[string]json.RawMessage{"error": json.RawMessage(`"supporter_key_not_accepted"`)}
	if err := service.providerError(http.StatusBadRequest, object, "supporter_key_not_accepted"); !errors.Is(err, ErrInvalid) || err.Error() != "supporter key was not accepted" {
		t.Fatalf("provider error = %v", err)
	}
}
