package supporter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type coverageFailedLoad struct{ memoryStore }

func (*coverageFailedLoad) LoadJSON(context.Context, string, any) (bool, error) {
	return false, errors.New("storage unavailable")
}

func TestCoverageSupporterConstructionFailures(t *testing.T) {
	if _, err := New(nil, nil, Config{}); !errors.Is(err, ErrInvalid) { //nolint:staticcheck // Verify the public nil-context rejection contract.
		t.Fatalf("nil context = %v", err)
	}
	if _, err := New(t.Context(), nil, Config{ActivationURL: "ftp://invalid.test"}); err == nil {
		t.Fatal("invalid activation endpoint accepted")
	}
	if _, err := New(t.Context(), &coverageFailedLoad{}, Config{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("failed load = %v", err)
	}
	service, err := New(t.Context(), nil, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Activate(t.Context(), ActivationInput{Key: "VALID_SUPPORTER_KEY"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("activation without storage = %v", err)
	}
}

func TestCoverageSupporterProviderFailurePreservesState(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(provider.Close)
	store := &memoryStore{}
	service, err := New(t.Context(), store, Config{ActivationURL: provider.URL, HTTPClient: provider.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Activate(t.Context(), ActivationInput{Key: "VALID_SUPPORTER_KEY"}); err == nil {
		t.Fatal("provider failure accepted")
	}
	if store.saves != 0 || service.Status().Active {
		t.Fatal("failed activation changed state")
	}
}
