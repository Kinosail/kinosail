package supporter

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestConfigurationAndEndpointValidation(t *testing.T) {
	app := App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"}
	for _, config := range []Config{
		{},
		{App: app, ActivationURL: "http://example.com/activate"},
		{App: app, ActivationURL: "https://user@example.com/activate"},
		{App: app, SupportURL: "http://127.0.0.1/support"},
		{App: App{ID: "Bad ID", Name: "Player", Audience: "bad", MasterworkName: "Full Sail"}},
		{App: App{ID: "kino-player", Name: "Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", Legacy: LegacyPolicy(255)}},
	} {
		if _, err := New(config); !errors.Is(err, ErrInvalid) {
			t.Fatalf("New(%#v) = %v", config, err)
		}
	}
	if !ValidEndpoint("https://support.example/path", false) || !ValidEndpoint("http://127.0.0.1/path", true) || ValidEndpoint("http://example.com/path", true) {
		t.Fatal("endpoint policy changed")
	}
}

func TestParseActivationJSONRejectsAmbiguousValues(t *testing.T) {
	valid, err := ParseActivationJSON([]byte(`{"key":"VALID_SUPPORTER_KEY","recognitionName":"Quiet Name"}`))
	if err != nil || valid.RecognitionName == nil || *valid.RecognitionName != "Quiet Name" {
		t.Fatalf("valid input = %#v, %v", valid, err)
	}
	for _, input := range []string{
		``, `[]`, `{}`, `{"key":null}`, `{"key":"VALID_SUPPORTER_KEY","key":"OTHER_SUPPORTER_KEY"}`,
		`{"key":"VALID_SUPPORTER_KEY","recognitionName":null}`, `{"key":"VALID_SUPPORTER_KEY","recognitionName":""}`,
		`{"key":"VALID_SUPPORTER_KEY","recognitionName":" padded "}`, `{"key":"VALID_SUPPORTER_KEY","unknown":true}`,
	} {
		if _, parseErr := ParseActivationJSON([]byte(input)); !errors.Is(parseErr, ErrInvalid) {
			t.Fatalf("ParseActivationJSON(%q) = %v", input, parseErr)
		}
	}
}

func TestStateValidationAndPreparationRejectMalformedState(t *testing.T) {
	service, err := New(Config{App: App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"}})
	if err != nil {
		t.Fatal(err)
	}
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	encodedCertificate := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
	encodedSignature := base64.RawURLEncoding.EncodeToString(make([]byte, 64))
	encodedHash := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	invalid := []State{
		{LivingLevel: 11},
		{InstallationKey: "invalid"},
		{InstallationKey: installation, PublicKey: "invalid"},
		{InstallationKey: installation, PatronOrder: &Grant{ActivationID: "invalid"}},
		{InstallationKey: installation, PatronOrder: &Grant{ActivationID: testActivationID, Certificate: encodedCertificate, Signature: encodedSignature, KeyHash: encodedHash}},
		{InstallationKey: installation, PatronOrder: &Grant{ActivationID: testActivationID, Certificate: encodedCertificate, Signature: encodedSignature, PublicKey: "invalid", KeyHash: encodedHash}},
		{InstallationKey: installation, Activations: [4]Activation{{ActivationID: testActivationID, KeyHash: "bad"}}},
	}
	for _, state := range invalid {
		if service.ValidateState(state) == nil {
			t.Fatalf("ValidateState(%#v) succeeded", state)
		}
	}
	partial := State{LivingLevel: 1}
	if next, prepareErr := service.Prepare(partial); prepareErr == nil || next.LivingLevel != partial.LivingLevel {
		t.Fatalf("Prepare(%#v) = %#v, %v", partial, next, prepareErr)
	}
	if !ValidRecognitionName("Visible Name", true) || ValidRecognitionName("\u2060Hidden", true) || ValidRecognitionName(strings.Repeat("x", 81), true) {
		t.Fatal("recognition name policy changed")
	}
}

func TestTierCatalogIsIsolated(t *testing.T) {
	first := Tiers()
	first[0] = "changed"
	if Tiers()[0] != "friend" || Rank("northstar") != 9 || Name("northstar") != "North Star" || Name("unknown") != "Free" {
		t.Fatal("tier catalog changed")
	}
}

func TestEmptyDerivativeStatusAndFingerprintUseSharedPolicy(t *testing.T) { //nolint:cyclop // One test covers the related empty derivative contract.
	service, err := New(Config{App: App{
		ID: "kino-subtitles", Name: "Kinosail Subtitles", Audience: "com.kinosail.subtitles", MasterworkName: "Perfect Sync",
		EmptyBadges: true, NestedApp: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	status := service.Status(State{})
	if status.App == nil || status.App.ID != "kino-subtitles" || status.PatronOrder == nil || status.LivingStandard == nil || status.PatronOrder.Tier != "free" {
		t.Fatalf("empty derivative status = %#v", status)
	}
	name := "Quiet Name"
	without := service.Fingerprint(ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	with := service.Fingerprint(ActivationInput{Key: "VALID_SUPPORTER_KEY", RecognitionName: &name})
	if without == "" || with == "" || without == with || !ValidKey("VALID_SUPPORTER_KEY") || ValidKey("short") {
		t.Fatal("activation fingerprint or key policy changed")
	}
}
