package federation

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSAMLMetadataRedirectsBoundTargetsBeforeBegin(t *testing.T) { //nolint:cyclop,funlen,gocognit // One public metadata-loading matrix proves target admission, request bounds, and durable no-key controls.
	metadata := samlMetadataDocument("http://localhost", "/sso", samlMetadataCertificate(t))
	for _, test := range []struct {
		name                                           string
		wantSuccess                                    bool
		wantOrigin, wantFinal, wantAllowed, wantDenied int
	}{
		{"direct", true, 1, 0, 0, 0},
		{"same-origin", true, 1, 1, 0, 0},
		{"nine-followed-redirects", true, 9, 1, 0, 0},
		{"twelve-followed-redirects", false, 10, 0, 0, 0},
		{"full-url-2048", true, 1, 1, 0, 0},
		{"full-url-2049", false, 1, 0, 0, 0},
		{"userinfo", false, 1, 0, 0, 0},
		{"malformed-query-escape", false, 1, 0, 0, 0},
		{"malformed-path-escape", false, 1, 0, 0, 0},
		{"unknown-scheme", false, 1, 0, 0, 0},
		{"missing-host", false, 1, 0, 0, 0},
		{"fragment", false, 1, 0, 0, 0},
		{"denied-ipv6-http-origin", false, 1, 0, 0, 0},
		{"allowed-other-ipv4-http-origin", true, 1, 0, 1, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var mutex sync.Mutex
			originRequests, finalRequests, allowedRequests, deniedRequests := 0, 0, 0, 0
			originControls, allowedControls, deniedControls := 0, 0, 0
			finalRequestURIBytes, finalFullURLBytes := 0, 0
			respond := func(writer http.ResponseWriter) {
				writer.Header().Set("Content-Type", "application/samlmetadata+xml")
				_, _ = io.WriteString(writer, metadata)
			}
			secondary := func(allowed bool) http.Handler {
				return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					mutex.Lock()
					if request.URL.Path == "/reachability-control" {
						if allowed {
							allowedControls++
						} else {
							deniedControls++
						}
					} else {
						if allowed {
							allowedRequests++
						} else {
							deniedRequests++
						}
					}
					mutex.Unlock()
					respond(writer)
				})
			}
			allowed := httptest.NewServer(secondary(true))
			t.Cleanup(allowed.Close)
			deniedListener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "[::1]:0")
			if err != nil {
				t.Fatalf("required reachable IPv6 denied-origin fixture unavailable: %v", err)
			}
			denied := httptest.NewUnstartedServer(secondary(false))
			_ = denied.Listener.Close()
			denied.Listener = deniedListener
			denied.Start()
			t.Cleanup(denied.Close)
			location := ""
			remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				mutex.Lock()
				if request.URL.Path == "/reachability-control" {
					originControls++
					mutex.Unlock()
					respond(writer)
					return
				}
				if request.URL.Path == "/final" || strings.HasPrefix(request.URL.Path, "/final/") {
					finalRequests++
					finalRequestURIBytes = len(request.URL.RequestURI())
					finalFullURLBytes = len("http://" + request.Host + request.URL.RequestURI())
					mutex.Unlock()
					respond(writer)
					return
				}
				originRequests++
				mutex.Unlock()
				if test.name == "direct" {
					respond(writer)
					return
				}
				next := location
				if strings.HasPrefix(request.URL.Path, "/hop/") {
					remaining, _ := strconv.Atoi(strings.TrimPrefix(request.URL.Path, "/hop/"))
					if remaining > 0 {
						next = "/hop/" + strconv.Itoa(remaining-1)
					} else {
						next = "/final"
					}
				}
				writer.Header().Set("Location", next)
				writer.WriteHeader(http.StatusFound)
			}))
			t.Cleanup(remote.Close)
			switch test.name {
			case "same-origin":
				location = remote.URL + "/final"
			case "nine-followed-redirects":
				location = "/hop/7"
			case "twelve-followed-redirects":
				location = "/hop/10"
			case "full-url-2048", "full-url-2049":
				size := 2048
				if test.name == "full-url-2049" {
					size++
				}
				prefix := remote.URL + "/final/"
				location = prefix + strings.Repeat("x", size-len(prefix))
			case "userinfo":
				location = strings.Replace(remote.URL, "http://", "http://qa_user:qa_password@", 1) + "/final"
			case "malformed-query-escape":
				location = "/final?invalid=%zz"
			case "malformed-path-escape":
				location = "/final/%zz"
			case "unknown-scheme":
				location = strings.Replace(remote.URL, "http://", "file://", 1) + "/final"
			case "missing-host":
				location = "https:///final"
			case "fragment":
				location = remote.URL + "/final#fragment"
			case "denied-ipv6-http-origin":
				location = denied.URL + "/final"
			case "allowed-other-ipv4-http-origin":
				location = allowed.URL + "/final"
			}
			for _, server := range []*httptest.Server{remote, allowed, denied} {
				request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/reachability-control", nil)
				if err != nil {
					t.Fatal(err)
				}
				response, err := server.Client().Do(request)
				if err != nil {
					t.Fatal(err)
				}
				body, readErr := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if response.StatusCode != http.StatusOK || readErr != nil || string(body) != metadata {
					t.Fatal("valid metadata sink reachability control failed")
				}
			}
			directory := t.TempDir()
			flow := NewSAML(SAMLConfig{MetadataURL: remote.URL + "/metadata", RootURL: "http://localhost", DataDir: directory})
			if !flow.Configured() {
				t.Fatal("fixture could not reach configured public Begin")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			start, beginErr := flow.Begin(ctx, "", "")
			entries, readErr := os.ReadDir(directory)
			if readErr != nil {
				t.Fatal(readErr)
			}
			info, keyErr := os.Stat(filepath.Join(directory, "saml_sp.pem"))
			keyCreated := keyErr == nil
			addresses := []string{remote.Listener.Addr().String(), allowed.Listener.Addr().String(), denied.Listener.Addr().String()}
			remote.Close()
			allowed.Close()
			denied.Close()
			http.DefaultTransport.(*http.Transport).CloseIdleConnections()
			listenersClosed := true
			for _, address := range addresses {
				connection, dialErr := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(t.Context(), "tcp", address)
				if dialErr == nil {
					listenersClosed = false
					_ = connection.Close()
				}
			}
			mutex.Lock()
			result := map[string]any{"case": test.name, "success": beginErr == nil, "expectedSuccess": test.wantSuccess, "originRequests": originRequests, "expectedOriginRequests": test.wantOrigin, "finalRequests": finalRequests, "expectedFinalRequests": test.wantFinal, "allowedOtherOriginRequests": allowedRequests, "expectedAllowedOtherOriginRequests": test.wantAllowed, "deniedOriginRequests": deniedRequests, "originControls": originControls, "allowedControls": allowedControls, "deniedControls": deniedControls, "locationBytes": len(location), "finalRequestURIBytes": finalRequestURIBytes, "finalFullURLBytes": finalFullURLBytes, "keyCreated": keyCreated, "privateDataEntries": len(entries), "listenersClosed": listenersClosed}
			encoded, _ := json.Marshal(result)
			t.Log("QA_SAML_REDIRECT_RESULT " + string(encoded))
			if (beginErr == nil) != test.wantSuccess || originRequests != test.wantOrigin || finalRequests != test.wantFinal || allowedRequests != test.wantAllowed || deniedRequests != test.wantDenied {
				t.Errorf("public Begin target admission: success=%t want=%t origin=%d want=%d final=%d want=%d allowed=%d want=%d denied=%d", beginErr == nil, test.wantSuccess, originRequests, test.wantOrigin, finalRequests, test.wantFinal, allowedRequests, test.wantAllowed, deniedRequests)
			}
			if test.wantSuccess {
				if start.RedirectURL == "" || !keyCreated || len(entries) != 1 || info.Mode().Perm() != 0o600 {
					t.Error("reachable public Begin did not create one private key and a login destination")
				}
			} else if keyCreated || len(entries) != 0 || start.RedirectURL != "" || len(start.PostHTML) != 0 {
				t.Error("rejected redirect caused public Begin key or result effects")
			}
			if originControls != 1 || allowedControls != 1 || deniedControls != 1 || !listenersClosed {
				t.Error("reachability or cleanup evidence failed")
			}
			if test.name == "full-url-2048" && finalFullURLBytes != 2048 {
				t.Error("full URL2048 positive control changed")
			}
			mutex.Unlock()
		})
	}
}
