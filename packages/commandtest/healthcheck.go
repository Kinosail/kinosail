package commandtest

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// HealthcheckConfiguredAuthHost verifies the CLI uses the configured HTTPS host.
func HealthcheckConfiguredAuthHost(t *testing.T, unusedAddress func(*testing.T) string, start func(*testing.T, []string) *exec.Cmd, stop func(*exec.Cmd), assert func(*testing.T, []string), timeout time.Duration) {
	address, dataDir := unusedAddress(t), t.TempDir()
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), "KINOSAIL_CLI_HELPER=1", "KINOSAIL_DATA_DIR="+dataDir, "KINOSAIL_LISTEN="+address, "KINOSAIL_AUTH_URL=https://media.home:"+port)
	command := start(t, environment)
	defer stop(command)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // Tests only.
	var response *http.Response
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+address+"/healthz", nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Host = "media.home:" + port
		response, err = client.Do(request)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("HTTPS did not become ready: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP = %d", response.StatusCode)
	}
	assert(t, environment)
}
