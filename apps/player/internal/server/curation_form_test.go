package server

import (
	"net/url"
	"strings"
	"testing"
)

func TestValidCurationItemFormAcceptsTransportCSRFField(t *testing.T) {
	if !validCurationItemForm(url.Values{"included": {"false"}, "_csrf": {"token"}}) {
		t.Fatal("expected the validated transport CSRF field to be ignored by semantic form validation")
	}
	for name, form := range map[string]url.Values{
		"unknown":        {"included": {"false"}, "extra": {"value"}},
		"multiple csrf":  {"included": {"false"}, "_csrf": {"one", "two"}},
		"oversized csrf": {"included": {"false"}, "_csrf": {strings.Repeat("x", 129)}},
	} {
		if validCurationItemForm(form) {
			t.Fatalf("%s form unexpectedly passed validation", name)
		}
	}
}
