package scimapp

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConfigurationKeysAndView(t *testing.T) { //nolint:cyclop // One view matrix covers every derived state.
	t.Parallel()
	keys := ConfigurationKeys()
	keys[0] = "changed"
	if ConfigurationKeys()[0] != TokenKey {
		t.Fatal("configuration keys exposed shared state")
	}
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	request := httptest.NewRequestWithContext(context.Background(), "GET", "http://media.example/settings/configuration", nil)
	request.Host = "media.example"
	view := configurationView(ConfigurationState{Origin: "https://configured.example/"}, "control", request, now, bytes.NewReader(make([]byte, 32)))
	if view.Endpoint != "https://configured.example/scim/v2" || view.ExpiresOn != "2026-12-02" || view.MinimumDate != "2026-09-04" || view.MaximumDate != "2026-12-03" || len(view.SuggestedToken) != 43 || view.Configured || view.Expired || view.Control != "control" {
		t.Fatalf("new view = %#v", view)
	}
	expiration := now.Add(time.Hour).Format(time.RFC3339)
	secure := httptest.NewRequestWithContext(context.Background(), "GET", "https://secure.example/settings/configuration", nil)
	configured := configurationView(ConfigurationState{Expiration: expiration, Configured: true}, 7, secure, now, strings.NewReader("unused"))
	if configured.Endpoint != "https://secure.example/scim/v2" || configured.ExpiresOn != "2026-09-04" || configured.SuggestedToken != "" || !configured.Configured || configured.Expired || configured.Control != 7 {
		t.Fatalf("configured view = %#v", configured)
	}
	failed := configurationView(ConfigurationState{}, false, request, now, errorReader{})
	if failed.SuggestedToken != "" || failed.Endpoint != "http://media.example/scim/v2" {
		t.Fatalf("failed token view = %#v", failed)
	}
	wrapped := NewConfigurationView(ConfigurationState{Configured: true}, true, request)
	if !wrapped.Configured || wrapped.Control != true {
		t.Fatalf("wrapped view = %#v", wrapped)
	}
}

func TestConfigurationExpiryAndEndpoint(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	if Expired("", now) || !Expired("invalid", now) || !Expired(now.Format(time.RFC3339), now) || Expired(now.Add(time.Second).Format(time.RFC3339), now) {
		t.Fatal("expiration classification is invalid")
	}
	request := httptest.NewRequestWithContext(context.Background(), "GET", "http://ignored.example", nil)
	request.Host = "fallback.example"
	if Endpoint("https://configured.example///", request) != "https://configured.example/scim/v2" || Endpoint("", request) != "http://fallback.example/scim/v2" {
		t.Fatal("endpoint derivation is invalid")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("random failed") }
