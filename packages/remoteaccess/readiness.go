package remoteaccess

import (
	"net/http"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

// ReadinessCheck reports one enforced public-access invariant.
type ReadinessCheck struct {
	ID      string `json:"id"`
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}

// Readiness reports whether every public-access invariant is satisfied.
type Readiness struct {
	Ready        bool             `json:"ready"`
	Checks       []ReadinessCheck `json:"checks"`
	NextSteps    []SetupStep      `json:"nextSteps"`
	Reachability string           `json:"reachability"`
	PublicURL    string           `json:"publicUrl,omitempty"`
	Stopped      bool             `json:"stopped"`
}

// SecurePublicReadiness evaluates Player's public-access readiness policy.
func SecurePublicReadiness(status Status, profiles []identitycore.Profile, profileErr error, now time.Time) Readiness {
	certificateExpires, certificateErr := time.Parse(time.RFC3339, status.CertificateExpires)
	remoteSignIn := false
	for _, profile := range profiles {
		remoteSignIn = remoteSignIn || !profile.Owner && !profile.Disabled && !profile.SCIMDeleted && profile.Remote && profile.Secured()
	}
	listenerMessage := "Public listener and DuckDNS update are healthy"
	if status.Isolation == "public-gateway" {
		listenerMessage = "Public application socket and DuckDNS update are ready; confirm the gateway using cellular data"
	}
	checks := []ReadinessCheck{
		{"authorization-state", profileErr == nil, "Profile and authorization state is readable"},
		{"public-https", status.Mode == "https", "Dedicated public HTTPS is configured"},
		{"remote-policy", status.Policy == "public-v1", "Only approved sign-in and Viewer routes are exposed"},
		{"listener", status.State == "ready", listenerMessage},
		{"certificate", certificateErr == nil && certificateExpires.After(now.Add(24*time.Hour)), "Public certificate is valid for more than 24 hours"},
		{"viewer-signin", remoteSignIn, "A remote-enabled Viewer has a passkey or authenticator for secure sign-in"},
		{"kill-switch", status.Mode == "https", "Persistent public-only kill switch is available"},
	}
	result := Readiness{Ready: true, Checks: checks, Stopped: status.Mode == "https" && status.State == "killed"}
	if status.Mode == "https" {
		result.PublicURL = publicSetupURL(status.Hostname)
	}
	for _, check := range checks {
		result.Ready = result.Ready && check.Ready
	}
	result.NextSteps = remoteSetupSteps(result)
	result.Reachability = "unverified"
	return result
}

const readinessViewSource = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Remote access setup · Kinosail __PRODUCT__</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=__ASSET__"></head><body class="settings-page"><main class="settings-shell"><a class="back" href="/settings#access">{{icon "back"}} Settings</a>
<header class="settings-intro"><h1>Watch away from home</h1><p>{{if .Stopped}}Public access is turned off.{{else if .Ready}}Server checks passed. Try a movie using cellular data next.{{else}}Set up your Server, then add one rule in your router.{{end}}</p></header>
<section class="wide" aria-labelledby="boundary-heading"><h2 id="boundary-heading">Your library, with Viewer access</h2><p>Public HTTPS lets approved Viewers browse and play their allowed Libraries, save viewing progress, and use enabled download features. Owner settings, Profile management, backups, and integrations stay on your home network or an explicitly paired <a href="/settings/management">private management device</a>.</p><details><summary>What does opening this port allow?</summary><p>Opening a port makes the sign-in page reachable from the internet. A stolen Viewer session can expose that Viewer's media and activity. Keep Kinosail and your router updated; no internet-facing service can promise protection from every server vulnerability.</p></details>{{if .PublicURL}}<p><strong>Your public address:</strong> <a href="{{.PublicURL}}" rel="noreferrer">{{.PublicURL}}</a></p>{{else}}<p>Your public address will appear here after you configure DuckDNS.</p>{{end}}</section>
<section class="wide" aria-labelledby="setup-heading"><h2 id="setup-heading">Set up access step by step</h2><ol>{{range .NextSteps}}<li><h3>{{.Title}}</h3><p>{{.Instruction}}</p>{{if .Fields}}<table><caption>Router rule for the standard HTTPS installation</caption><thead><tr><th scope="col">Router field</th><th scope="col">What to enter</th></tr></thead><tbody>{{range .Fields}}<tr><th scope="row">{{.Label}}</th><td>{{.Value}}</td></tr>{{end}}</tbody></table><p>Use port 443 even if a router guide uses a different example.</p>{{end}}{{if .Links}}<details><summary>Find instructions for your router</summary><p>Official manufacturer guides. Use your router model's instructions and the Kinosail values above.</p><ul>{{range .Links}}<li><a href="{{.URL}}" target="_blank" rel="noopener noreferrer">{{.Label}} port-forwarding guide (opens a new tab)</a></li>{{end}}</ul><p>Router missing? Check the model on its label and search the manufacturer's support site for “port forwarding.” An internet-provider router may need its provider's app.</p></details>{{end}}</li>{{end}}</ol></section>
<section class="wide" aria-labelledby="checks-heading"><h2 id="checks-heading">Check your Server</h2><p>Outside access is unverified. Server checks cannot confirm that your router or internet provider allows incoming connections.</p><a class="button" href="/settings/remote-readiness">Check again</a>{{range .Checks}}<p><strong>{{if .Ready}}Passed{{else}}Needs attention{{end}}</strong> · {{.Message}}</p>{{end}}</section>
<section class="wide" aria-labelledby="help-heading"><h2 id="help-heading">If it does not connect</h2><details><summary>The connection times out</summary><p>Check the selected Server device, TCP 443 in both port fields, and the Server firewall. Confirm DuckDNS points to your current public IP address. If port 443 is already used, resolve the conflict before opening anything else.</p><p>If you have a router behind an internet-provider router, both need the rule: forward TCP 443 on the outer router to the inner router's WAN address, then on the inner router to the Server. Ask your provider for help if you are unsure.</p><p>Ask your internet provider: “Do I have a public IPv4 address, and can I receive incoming TCP 443 connections?” A private WAN address or 100.64.0.0–100.127.255.255 can mean double NAT or carrier-grade NAT (CGNAT). Port forwarding alone cannot fix CGNAT; ask for a public address or use an owner-controlled VPN that supports your network.</p></details><details><summary>A certificate warning appears</summary><p>Stop. Check that you used your exact DuckDNS HTTPS address and that TCP 443 reaches the Server's public HTTPS port. Wait for certificate issuance and check the error in Settings. Never bypass a warning at the public address.</p></details><details><summary>The page opens, but sign-in fails</summary><p>Use a remote-enabled Viewer, not an Owner. Password-only public sign-in is disabled. In a browser, choose Get a sign-in code. You can also use Quick Connect in the Kinosail app or a compatible Jellyfin app. Approve the code from a local Viewer session secured with an authenticator or passkey. Enable Jellyfin apps in Settings if you use one. Approvals and adding passkeys stay local; do not try to enroll a new passkey on the public listener.</p></details></section>
<section class="wide" aria-labelledby="stop-heading"><h2 id="stop-heading">Turn it off whenever you need</h2><p>At home, open <a href="/settings#access">Settings → Watch away from home</a> and choose Disable public access now. This stops public viewing, revokes public sessions, and keeps public access off after restart. Remove the Kinosail port-forwarding rule in your router too when you no longer need it. Local access stays available.</p></section></main></body></html>`

// ReadinessViewSource derives a branded view from Player's canonical markup.
func ReadinessViewSource(product, assetVersion string) string {
	if !safeReadinessToken(product, 32, false) || !safeReadinessToken(assetVersion, 8, true) {
		panic("invalid remote readiness view configuration")
	}
	return strings.NewReplacer("__PRODUCT__", product, "__ASSET__", assetVersion).Replace(readinessViewSource)
}

func safeReadinessToken(value string, maximum int, digitsOnly bool) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if !readinessCharacterAllowed(character, digitsOnly) {
			return false
		}
	}
	return true
}

func readinessCharacterAllowed(character rune, digitsOnly bool) bool {
	if digitsOnly {
		return character >= '0' && character <= '9'
	}
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

type readinessView interface {
	Execute(http.ResponseWriter, *http.Request, any) error
}

// NewReadinessHandler serves the shared readiness model through an app-local view.
func NewReadinessHandler(internet *Manager, profiles func() ([]identitycore.Profile, error), view readinessView, renderError func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	if profiles == nil || view == nil || renderError == nil {
		panic("invalid remote readiness dependencies")
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		status := Status{State: "disabled", Mode: "off"}
		if internet != nil {
			status = internet.Status()
		}
		availableProfiles, profileErr := profiles()
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := view.Execute(writer, request, SecurePublicReadiness(status, availableProfiles, profileErr, time.Now())); err != nil {
			renderError(writer, request, "remote readiness view failed", http.StatusInternalServerError)
		}
	}
}
