package scim

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSCIMFilterRejectsMalformedCompoundAndSingleExpressions(t *testing.T) { //nolint:cyclop // Compound, path, operator, and value grammar failures are independent.
	t.Parallel()
	for name, raw := range map[string]string{
		"unclosed quote":            `userName eq "viewer`,
		"empty part":                `userName eq "viewer" and `,
		"missing separator":         `userName`,
		"unsupported path":          `unknown eq "viewer"`,
		"invalid active":            `active sw true`,
		"invalid text":              `displayName eq true`,
		"invalid prepared username": `userName eq "a b"`,
	} {
		t.Run(name, func(t *testing.T) {
			if filter, err := scimFilter(raw); err == nil || filter != nil {
				t.Fatalf("invalid filter returned function=%v, error=%v", filter != nil, err)
			}
		})
	}
	if _, err := scimAndFilter([]string{`userName eq "viewer"`, `invalid`}); err == nil {
		t.Fatal("invalid compound filter part accepted")
	}
	if _, _, err := scimFilterPath("bad:path"); err == nil {
		t.Fatal("invalid schema-qualified filter path accepted")
	}
	if _, _, err := parseSCIMFilterComparison("displayname", "eq"); err == nil {
		t.Fatal("filter without comparison value accepted")
	}
	if _, err := splitSCIMAnd(` and userName eq "viewer"`); err == nil {
		t.Fatal("compound filter with an empty first clause accepted")
	}
}

func TestSCIMCompoundFilterShortCircuitsAndAttributeFallbacks(t *testing.T) {
	t.Parallel()
	filter, err := scimFilter(`userName eq "viewer" and displayName eq "Viewer"`)
	if err != nil {
		t.Fatal(err)
	}
	if filter(viewerProfile{UserName: "other", Name: "Viewer"}) {
		t.Fatal("compound filter matched after a failed clause")
	}
	if end := scimFilterAttributeEnd("userName"); end != -1 {
		t.Fatalf("attribute end = %d", end)
	}
	if values := scimFilterValues(viewerProfile{}, "displayname", scimPatchPath{}); values != nil {
		t.Fatalf("empty filter values = %#v", values)
	}
	if got := scimFilterAttribute(viewerProfile{ID: "profile"}, "id"); got != "profile" {
		t.Fatalf("ID filter value = %q", got)
	}
}

func TestSCIMQueryValuesRejectLengthEncodingAndSelectionEdges(t *testing.T) { //nolint:cyclop // Query size, encoding, value cardinality, and empty selection fail before use.
	t.Parallel()
	tooLong := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?value="+strings.Repeat("x", maxSCIMQueryLength), nil)
	if _, err := scimQueryValues(tooLong, "value"); err == nil {
		t.Fatal("oversized SCIM query accepted")
	}
	malformed := &http.Request{URL: &url.URL{RawQuery: "%zz"}}
	if _, err := scimQueryValues(malformed); err == nil {
		t.Fatal("malformed SCIM query accepted")
	}
	for name, rawURL := range map[string]string{
		"list included": "/?attributes=",
		"list excluded": "/?excludedAttributes=",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
			if _, err := scimListOptions(request); err == nil {
				t.Fatal("empty list selection accepted")
			}
		})
	}
	for name, rawURL := range map[string]string{
		"resource query":    "/?unknown=true",
		"resource included": "/?attributes=",
		"resource excluded": "/?excludedAttributes=",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
			if _, err := scimResourceOptions(request); err == nil {
				t.Fatal("invalid resource options accepted")
			}
		})
	}
}

func TestSCIMPageAndAttributeSelectionRejectMalformedValues(t *testing.T) { //nolint:cyclop // Numeric parsing and attribute path normalization fail closed.
	t.Parallel()
	if _, _, err := scimListPage(url.Values{"count": {"many"}}); err == nil {
		t.Fatal("non-numeric page size accepted")
	}
	for name, call := range map[string]func() error{
		"included": func() error {
			_, err := scimSelectionFrom("bad:path", "")
			return err
		},
		"excluded": func() error {
			_, err := scimSelectionFrom("", "bad:path")
			return err
		},
		"duplicate attribute": func() error {
			_, err := scimAttributeSet("userName,USERNAME")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid attribute selection accepted")
			}
		})
	}
}

func TestSCIMCanonicalPathSchemaEdges(t *testing.T) { //nolint:cyclop // Core, enterprise, and foreign schema path forms remain distinct.
	t.Parallel()
	for raw, want := range map[string]string{
		scimEnterpriseUser:                 "enterprise",
		scimEnterpriseUser + ":department": "enterprise.department",
		scimUserSchema + ":userName":       "username",
	} {
		if got, err := canonicalSCIMPath(raw); err != nil || got != want {
			t.Fatalf("canonical path %q = %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"urn:example:User:userName", scimUserSchema + ":bad:path"} {
		if _, err := canonicalSCIMPath(raw); err == nil {
			t.Fatalf("foreign path %q accepted", raw)
		}
	}
	for raw, want := range map[string]string{
		scimUserSchema + ":displayName":    "displayname",
		scimEnterpriseUser + ":department": "enterprise.department",
	} {
		if got, err := canonicalSCIMPatchPath(raw); err != nil || got != want {
			t.Fatalf("canonical patch path %q = %q, %v", raw, got, err)
		}
	}
	if _, err := canonicalSCIMPatchPath("urn:example:User:userName"); err == nil {
		t.Fatal("foreign patch path accepted")
	}
	if _, err := prepareUserName(""); err == nil {
		t.Fatal("blank username preparation accepted")
	}
}
