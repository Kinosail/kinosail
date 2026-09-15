package settings

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDLNATokenAndLabelPolicy(t *testing.T) { //nolint:cyclop,gocognit // The policy matrix remains below the repository ceiling of 22.
	t.Parallel()
	if token, changed := ConfiguredDLNAToken(false, true, "existing"); token != "existing" || changed {
		t.Fatalf("unmanaged token = %q, %t", token, changed)
	}
	if token, changed := ConfiguredDLNAToken(true, false, "existing"); token != "" || !changed {
		t.Fatalf("disabled token = %q, %t", token, changed)
	}
	if token, changed := ConfiguredDLNAToken(true, true, "existing"); token != "existing" || !changed {
		t.Fatalf("retained token = %q, %t", token, changed)
	}
	if token, changed := ConfiguredDLNAToken(true, true, ""); token == "" || !changed {
		t.Fatalf("generated token = %q, %t", token, changed)
	}
	if token, err := NewDLNAToken(false, ""); token != "" || err != nil {
		t.Fatalf("disabled token = %q, %v", token, err)
	}
	if token, err := NewDLNAToken(true, "invalid"); token != "" || err == nil {
		t.Fatalf("invalid address token = %q, %v", token, err)
	}
	if token, err := NewDLNAToken(true, "http://192.168.1.2:8080"); token == "" || err != nil {
		t.Fatalf("new token = %q, %v", token, err)
	}
	if DLNALabel("", "") != "Set KINOSAIL_DLNA_URL to enable" || DLNALabel("http://lan", "token") != "Enabled" || DLNALabel("http://lan", "") != "Disabled" {
		t.Fatal("DLNA labels changed")
	}
}

func TestSaveDLNAValidatesAndAppliesChoice(t *testing.T) { //nolint:cyclop // One table protects every form boundary and result.
	t.Parallel()
	for _, test := range []struct {
		name      string
		values    url.Values
		changeErr error
		wantCode  int
		wantCalls int
		wantValue bool
	}{
		{"unknown", url.Values{"extra": {"true"}}, nil, http.StatusBadRequest, 0, false},
		{"duplicate", url.Values{"enabled": {"true", "false"}}, nil, http.StatusBadRequest, 0, false},
		{"value", url.Values{"enabled": {"yes"}}, nil, http.StatusBadRequest, 0, false},
		{"change", url.Values{"enabled": {"true"}}, errors.New("save failed"), http.StatusBadRequest, 1, true},
		{"enable", url.Values{"enabled": {"true"}}, nil, http.StatusSeeOther, 1, true},
		{"disable", url.Values{"enabled": {"false"}}, nil, http.StatusSeeOther, 1, false},
	} {
		recorder, failures, calls, value := httptest.NewRecorder(), &errorRecord{}, 0, false
		SaveDLNA(func(enabled bool) error { calls++; value = enabled; return test.changeErr }, failures.write)(recorder, formRequest("/settings/dlna", test.values))
		if recorder.Code != test.wantCode || calls != test.wantCalls || value != test.wantValue || failures.calls != boolInt(test.wantCode == http.StatusBadRequest) {
			t.Fatalf("%s code=%d calls=%d value=%t failures=%#v", test.name, recorder.Code, calls, value, failures)
		}
	}
}
