package remoteaccess

import (
	"bytes"
	"html/template"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSetupViewRendersAccessibleRouterInstructions(t *testing.T) {
	t.Parallel()
	view := template.Must(template.New("setup").Funcs(template.FuncMap{"icon": func(string) string { return "" }}).Parse(ReadinessViewSource("Player", "75")))
	var output bytes.Buffer
	data := SecurePublicReadiness(Status{Mode: "https", Hostname: "family.duckdns.org"}, nil, nil, time.Now())
	if err := view.Execute(&output, data); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Watch away from home", `scope="col"`, `scope="row"`, "TCP only", "443", "Find instructions for your router", "Outside access is unverified", `href="https://family.duckdns.org"`, "carrier-grade NAT", "Disable public access now", "Quick Connect", "Never bypass"} {
		if !strings.Contains(output.String(), text) {
			t.Errorf("missing rendered guidance: %q", text)
		}
	}
	for _, guide := range routerGuides() {
		parsed, err := url.Parse(guide.URL)
		if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" || guide.Label == "" {
			t.Fatalf("invalid guide: %#v", guide)
		}
		switch parsed.Host {
		case "www.tp-link.com", "www.asus.com", "kb.netgear.com", "eero.com", "support.google.com", "www.xfinity.com":
		default:
			t.Fatalf("untrusted guide host: %s", parsed.Host)
		}
	}
	if strings.Contains(output.String(), "YOUR-NAME") || strings.Contains(output.String(), "#ZgotmplZ") {
		t.Fatal("setup rendered a placeholder or unsafe link")
	}
}

func TestSetupViewEscapesFieldAndLinkLabels(t *testing.T) {
	t.Parallel()
	view := template.Must(template.New("setup").Funcs(template.FuncMap{"icon": func(string) string { return "" }}).Parse(ReadinessViewSource("Player", "75")))
	data := Readiness{NextSteps: []SetupStep{{Title: "<script>attack()</script>", Fields: []SetupField{{Label: "<img src=x>", Value: "<script>attack()</script>"}}, Links: []SetupLink{{Label: "<svg onload=attack()>", URL: "https://www.asus.com/"}}}}}
	var output bytes.Buffer
	if err := view.Execute(&output, data); err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"<script>attack()", "<img src=x>", "<svg onload="} {
		if strings.Contains(output.String(), unsafe) {
			t.Fatalf("unescaped setup content %q", unsafe)
		}
	}
}
