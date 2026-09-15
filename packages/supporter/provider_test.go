package supporter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestProviderResponsesFailClosedWithoutStateChange(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		status                  int
		want                    error
	}{
		{"wrong content type", "text/plain", `{}`, http.StatusOK, ErrUnavailable},
		{"unknown success field", "application/json", `{"certificate":"x","signature":"x","publicKey":"x","activationId":"x","extra":true}`, http.StatusOK, ErrInvalid},
		{"duplicate error", "application/json", `{"error":"invalid_request","error":"invalid_request"}`, http.StatusBadRequest, ErrUnavailable},
		{"player invalid request", "application/json", `{"error":"invalid_request"}`, http.StatusBadRequest, ErrUnavailable},
		{"expired key", "application/json", `{"error":"supporter_key_expired"}`, http.StatusBadRequest, ErrInvalid},
		{"server key error", "application/json", `{"error":"supporter_key_expired"}`, http.StatusInternalServerError, ErrUnavailable},
		{"provider failure", "application/json", `{"error":"provider_unavailable"}`, http.StatusBadGateway, ErrUnavailable},
		{"oversized", "application/json", strings.Repeat("x", 8193), http.StatusOK, ErrUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", test.contentType)
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			service, err := New(Config{
				App:           App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"},
				ActivationURL: server.URL, HTTPClient: server.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			state, _ := service.Prepare(State{})
			next, _, activateErr := service.Activate(context.Background(), state, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
			if !errors.Is(activateErr, test.want) || !reflect.DeepEqual(next, state) {
				t.Fatalf("Activate = %#v, %v", next, activateErr)
			}
		})
	}
}

func TestDerivativeInvalidRequestResponseFailsClosedWithoutStateChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalid_request"}`))
	}))
	defer server.Close()
	service, err := New(Config{
		App: App{
			ID: "kino-example", Name: "Example App", Audience: "com.example.app", MasterworkName: "Example Masterwork", AcceptInvalidRequestError: true,
		},
		ActivationURL: server.URL, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	state, _ := service.Prepare(State{})
	next, _, activateErr := service.Activate(context.Background(), state, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if !errors.Is(activateErr, ErrInvalid) || !reflect.DeepEqual(next, state) {
		t.Fatalf("Activate = %#v, %v", next, activateErr)
	}
}
