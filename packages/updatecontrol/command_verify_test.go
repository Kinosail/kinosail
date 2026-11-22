package updatecontrol

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
)

type recordedClient struct {
	requests  []string
	responses map[string]*http.Response
	err       error
}

func (client *recordedClient) Do(request *http.Request) (*http.Response, error) {
	client.requests = append(client.requests, request.URL.String())
	if request.Header.Get("User-Agent") != "Kinosail-update-check" || request.Header.Get("Cookie") != "" || request.Header.Get("Authorization") != "" {
		return nil, errors.New("unexpected request metadata")
	}
	if client.err != nil {
		return nil, client.err
	}
	if response, ok := client.responses[request.URL.String()]; ok {
		return response, nil
	}
	return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("missing"))}, nil
}

func okResponse(body []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
}

func verifyCommand(t *testing.T, client HTTPClient, material func(string) (root.TrustedMaterial, error)) Command {
	t.Helper()
	return BindCommand(CommandConfig{
		Policy:       PlayerPolicy(1, 1),
		OpenDatabase: func(string) (CommandDatabase, error) { return nil, errors.New("not used") },
		Version:      func() string { return "v0.0.1" },
		Trust:        TrustConfig{Client: client, Now: func() time.Time { return channelSigned }, Material: material},
	})
}

func TestUpdateVerifyRejectsUnsignedOrForgedChannelsBeforeFetchingReleases(t *testing.T) {
	t.Parallel()
	material := publicGoodRoot(t)
	channel := "https://github.com/Kinosail/kinosail/releases/download/player-channel/channel.json"
	releaseSignature := readFixture(t, "player-v0.1.0/kinosail-release.json.sigstore.json")
	cases := map[string]map[string]*http.Response{
		"missing channel":            {},
		"unsigned channel":           {channel: okResponse(channelStatement("latest", `"v0.1.0"`))},
		"release signature replayed": {channel: okResponse(channelStatement("latest", `"v0.1.0"`)), channel + ".sigstore.json": okResponse(releaseSignature)},
		"oversized channel":          {channel: okResponse(bytes.Repeat([]byte("a"), signedInputLimit+1)), channel + ".sigstore.json": okResponse(releaseSignature)},
		"server error":               {channel: {StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("{}"))}},
	}
	for name, responses := range cases {
		client := &recordedClient{responses: responses}
		var output bytes.Buffer
		err := verifyCommand(t, client, func(dataDir string) (root.TrustedMaterial, error) {
			if dataDir != "/data" {
				t.Fatalf("trust root data path = %q", dataDir)
			}
			return material, nil
		})(CommandVerify, nil, &output, "/data", "linux", "amd64")
		if err == nil || output.Len() != 0 {
			t.Errorf("%s = %q, %v", name, output.String(), err)
		}
		for _, requested := range client.requests {
			if requested != channel && requested != channel+".sigstore.json" {
				t.Errorf("%s fetched %s before trusting the channel", name, requested)
			}
		}
	}
}

func TestUpdateVerifyFailsClosedWithoutTrustRootStorageOrNetwork(t *testing.T) {
	t.Parallel()
	client := &recordedClient{}
	unavailable := verifyCommand(t, client, func(string) (root.TrustedMaterial, error) { return nil, errors.New("offline") })
	var output bytes.Buffer
	if err := unavailable(CommandVerify, nil, &output, "/data", "linux", "amd64"); err == nil || len(client.requests) != 0 || output.Len() != 0 {
		t.Fatalf("missing trust root = %v, requests %v", err, client.requests)
	}
	material := publicGoodRoot(t)
	available := verifyCommand(t, client, func(string) (root.TrustedMaterial, error) { return material, nil })
	if err := available(CommandVerify, nil, &output, "", "linux", "amd64"); err == nil || len(client.requests) != 0 {
		t.Fatalf("missing data directory = %v", err)
	}
	if err := available(CommandVerify, nil, nil, "/data", "linux", "amd64"); err == nil || len(client.requests) != 0 {
		t.Fatalf("missing output = %v", err)
	}
	offline := &recordedClient{err: errors.New("dial failed")}
	if err := verifyCommand(t, offline, func(string) (root.TrustedMaterial, error) { return material, nil })(CommandVerify, nil, &output, "/data", "linux", "amd64"); err == nil || output.Len() != 0 {
		t.Fatalf("offline = %v", err)
	}
}
