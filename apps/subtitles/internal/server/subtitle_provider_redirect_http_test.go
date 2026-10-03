package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSubtitleCredentialRedirectsBoundTargetsBeforeRequest(t *testing.T) { //nolint:cyclop,funlen,gocognit // One real HTTP matrix records target rejection, admitted credentials, query preservation, and listener cleanup.
	const key = "qa_synthetic_provider_key"
	for _, providerMode := range []string{"SubSource", "SubDL", "SubDL-reconfigured", "OpenSubtitles"} {
		for _, test := range []struct {
			name                                                 string
			wantConnected, wantOriginRequests, wantFinalRequests int
		}{
			{"direct", 1, 1, 0},
			{"same-origin", 1, 1, 1},
			{"nine-followed-redirects", 1, 9, 1},
			{"tenth-redirect-attempt", 0, 10, 0},
			{"full-url-4096", 1, 1, 1},
			{"full-url-4097", 0, 1, 0},
			{"userinfo", 0, 1, 0},
			{"unknown-scheme", 0, 1, 0},
			{"missing-host", 0, 1, 0},
			{"malformed-location", 0, 1, 0},
			{"malformed-query-escape", 0, 1, 0},
			{"valid-percent-escaped-query", 1, 1, 1},
			{"semicolon-query", 1, 1, 1},
			{"foreign-origin", 0, 1, 0},
		} {
			t.Run(providerMode+"/"+test.name, func(t *testing.T) {
				providerName := providerMode
				reconfigured := providerName == "SubDL-reconfigured"
				if reconfigured {
					providerName = "SubDL"
				}

				var mutex sync.Mutex
				originRequests, finalRequests, foreignRequests, originControls, foreignControls := 0, 0, 0, 0, 0
				originKeyHeaders, originQueryKeys, finalKeyHeaders, finalRefererKeys := 0, 0, 0, 0
				finalRequestURIBytes, finalReconstructedURLBytes := 0, 0
				finalRawQuery, expectedRawQuery := "", ""
				respond := func(writer http.ResponseWriter) {
					if providerName == "OpenSubtitles" {
						_, _ = io.WriteString(writer, `{"status":200,"token":"qa_synthetic_provider_bearer"}`)
						return
					}
					_, _ = io.WriteString(writer, `{}`)
				}
				outside := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					mutex.Lock()
					if request.URL.Path == "/reachability-control" {
						foreignControls++
					} else {
						foreignRequests++
					}
					mutex.Unlock()
					respond(writer)
				}))
				t.Cleanup(outside.Close)
				location := ""
				remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					mutex.Lock()
					if request.URL.Path == "/reachability-control" {
						originControls++
						mutex.Unlock()
						respond(writer)
						return
					}
					if request.URL.Path == "/final" {
						finalRequests++
						finalRequestURIBytes = len(request.URL.RequestURI())
						finalReconstructedURLBytes = len("http://" + request.Host + request.URL.RequestURI())
						finalRawQuery = request.URL.RawQuery
						if request.Header.Get("X-API-Key") == key || request.Header.Get("Api-Key") == key {
							finalKeyHeaders++
						}
						if strings.Contains(request.Header.Get("Referer"), key) {
							finalRefererKeys++
						}
						mutex.Unlock()
						respond(writer)
						return
					}
					originRequests++
					if request.Header.Get("X-API-Key") == key || request.Header.Get("Api-Key") == key {
						originKeyHeaders++
					}
					if request.URL.Query().Get("api_key") == key {
						originQueryKeys++
					}
					mutex.Unlock()
					if test.name == "direct" {
						respond(writer)
						return
					}
					next := location
					if request.URL.Path == "/hop" {
						remaining, _ := strconv.Atoi(request.URL.Query().Get("remaining"))
						if remaining > 0 {
							next = "/hop?remaining=" + strconv.Itoa(remaining-1)
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
					location = "/hop?remaining=7"
				case "tenth-redirect-attempt":
					location = "/hop?remaining=8"
				case "full-url-4096", "full-url-4097":
					size := 4096
					if test.name == "full-url-4097" {
						size++
					}
					prefix := remote.URL + "/final?pad="
					location = prefix + strings.Repeat("x", size-len(prefix))
				case "userinfo":
					location = strings.Replace(remote.URL, "http://", "http://qa_synthetic_user:qa_synthetic_password@", 1) + "/final"
				case "unknown-scheme":
					location = strings.Replace(remote.URL, "http://", "file://", 1) + "/final"
				case "missing-host":
					location = "https:///final"
				case "malformed-location":
					location = "/final/%zz"
				case "malformed-query-escape":
					location = "/final?invalid=%zz"
				case "valid-percent-escaped-query":
					expectedRawQuery = "value=qa%3Asafe%2Fvalue%2Bwith%20space&percent=%25zz"
					location = "/final?" + expectedRawQuery
				case "semicolon-query":
					expectedRawQuery = "value=first;second&other=safe"
					location = "/final?" + expectedRawQuery
				case "foreign-origin":
					location = outside.URL + "/final"
				}
				// The forbidden sinks are independently reachable with valid JSON and no credentials.
				for _, server := range []*httptest.Server{outside, remote} {
					request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/reachability-control", nil)
					if err != nil {
						t.Fatal(err)
					}
					response, err := server.Client().Do(request)
					if err != nil {
						t.Fatal(err)
					}
					_, _ = io.Copy(io.Discard, response.Body)
					_ = response.Body.Close()
					if response.StatusCode != http.StatusOK {
						t.Fatal("sink reachability control failed")
					}
				}
				config := SubtitleConfig{}
				switch providerName {
				case "SubSource":
					config.SubSource = SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: key, PersonalUse: true}
				case "SubDL":
					config.URL, config.APIKey = remote.URL+"/api/v1", key
				case "OpenSubtitles":
					config.OpenSubtitles = OpenSubtitlesConfig{URL: remote.URL + "/api/v1", APIKey: key, Username: "qa_synthetic_username", Password: "qa_synthetic_password"}
				}
				initialConfig := config
				if reconfigured {
					initialConfig.APIKey += "-previous"
					initialConfig.URL = outside.URL + "/api/v1"
				}
				provider := newSubtitleProvider(initialConfig, t.TempDir(), t.TempDir(), nil, nil, "")
				if reconfigured {
					provider.replaceConfig(config)
					provider.client.CloseIdleConnections()
					provider.subsource.client.CloseIdleConnections()
					provider.open.client.CloseIdleConnections()
					provider = provider.active()
				}
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				attempted, connected := provider.testCredentials(ctx)
				provider.client.CloseIdleConnections()
				provider.subsource.client.CloseIdleConnections()
				provider.open.client.CloseIdleConnections()
				addresses := []string{remote.Listener.Addr().String(), outside.Listener.Addr().String()}
				remote.Close()
				outside.Close()
				listenersClosed := true
				for _, address := range addresses {
					c, e := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(t.Context(), "tcp", address)
					if e == nil {
						listenersClosed = false
						_ = c.Close()
					}
				}
				mutex.Lock()
				result := map[string]any{"provider": providerMode, "case": test.name, "attempted": attempted, "connected": connected, "expectedConnected": test.wantConnected, "originRequests": originRequests, "expectedOriginRequests": test.wantOriginRequests, "finalRequests": finalRequests, "expectedFinalRequests": test.wantFinalRequests, "foreignRequests": foreignRequests, "originReachabilityControls": originControls, "foreignReachabilityControls": foreignControls, "originKeyHeaders": originKeyHeaders, "originQueryKeys": originQueryKeys, "finalKeyHeaders": finalKeyHeaders, "finalRefererKeys": finalRefererKeys, "locationBytes": len(location), "finalRequestURIBytes": finalRequestURIBytes, "finalReconstructedURLBytes": finalReconstructedURLBytes, "listenersClosed": listenersClosed}
				if expectedRawQuery != "" || test.name == "malformed-query-escape" {
					result["observedRawQuery"] = finalRawQuery
					result["queryWireControlPreserved"] = expectedRawQuery != "" && finalRawQuery == expectedRawQuery
				}
				encoded, _ := json.Marshal(result)
				t.Log("QA_PROVIDER_SHIPPING_RESULT " + string(encoded))
				if attempted != 1 || connected != test.wantConnected || originRequests != test.wantOriginRequests || finalRequests != test.wantFinalRequests || foreignRequests != 0 {
					t.Errorf("redirect operation: attempted=%d connected=%d want=%d origin=%d want=%d final=%d want=%d foreign=%d", attempted, connected, test.wantConnected, originRequests, test.wantOriginRequests, finalRequests, test.wantFinalRequests, foreignRequests)
				}
				if originControls != 1 || foreignControls != 1 || !listenersClosed {
					t.Error("reachability or cleanup evidence failed")
				}
				if providerName == "SubDL" {
					if originQueryKeys != 1 {
						t.Error("query key control did not reach origin")
					}
				} else if originKeyHeaders != originRequests {
					t.Error("header key control did not reach each admitted origin request")
				}
				if test.wantFinalRequests == 1 && providerName != "SubDL" && finalKeyHeaders != 1 {
					t.Error("same-origin key header control did not reach final target")
				}
				if test.name == "full-url-4096" && finalReconstructedURLBytes != 4096 {
					t.Error("positive full URL size control changed")
				}
				if expectedRawQuery != "" && finalRawQuery != expectedRawQuery {
					t.Error("accepted query encoding changed on the wire")
				}
				mutex.Unlock()
			})
		}
	}
}
