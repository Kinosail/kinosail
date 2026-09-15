package trustedhttps

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
)

func TestIssueCompletesDNSChallengeAndEncodesIdentity(t *testing.T) { //nolint:cyclop // One test proves the complete ACME adapter exchange.
	now := time.Now()
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	queries := make([]url.Values, 0, 2)
	duckDNS := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queries = append(queries, request.URL.Query())
		_, _ = writer.Write([]byte("OK"))
	}))
	defer duckDNS.Close()
	client := newFakeACME(t, now)
	waited := false
	provider := duckDNSProvider{config: config, client: duckDNS.Client(), endpoint: duckDNS.URL}
	identity, err := issue(t.Context(), config, client, provider, func(_ context.Context, name, value string) error {
		waited = true
		if name != "_acme-challenge."+config.Hostname() || value != "dns-proof" {
			t.Fatalf("DNS wait = %q %q", name, value)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair(identity, identity)
	if err != nil || len(certificate.Certificate) != 1 {
		t.Fatalf("identity = %v, certificates=%d", err, len(certificate.Certificate))
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil || leaf.VerifyHostname(config.Hostname()) != nil || !waited || !client.accepted {
		t.Fatalf("issued identity = %v, waited=%v accepted=%v", err, waited, client.accepted)
	}
	if len(queries) != 2 {
		t.Fatalf("DuckDNS requests = %d", len(queries))
	}
	assertDuckDNSQuery(t, queries[0], config, "", "dns-proof", false)
	assertDuckDNSQuery(t, queries[1], config, "", "", true)
}

func TestAccountKeyPersistsAndRejectsInvalidMaterial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", accountKeyName)
	first, err := accountKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := accountKey(path)
	if err != nil || !first.Equal(second) {
		t.Fatalf("reloaded key = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("account key mode = %v, %v", info, err)
	}
	if err = os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = accountKey(path); err == nil {
		t.Fatal("invalid account key was accepted")
	}
}

func TestIssueRejectsFailedAuthorizationWithoutLeavingTXTRecord(t *testing.T) {
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	queries := make([]url.Values, 0, 2)
	duckDNS := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queries = append(queries, request.URL.Query())
		_, _ = writer.Write([]byte("OK"))
	}))
	defer duckDNS.Close()
	client := newFakeACME(t, time.Now())
	client.fail = "accept"
	provider := duckDNSProvider{config: config, client: duckDNS.Client(), endpoint: duckDNS.URL}
	if _, err := issue(t.Context(), config, client, provider, func(context.Context, string, string) error { return nil }); err == nil {
		t.Fatal("failed challenge was accepted")
	}
	if len(queries) != 2 || queries[1].Get("clear") != "true" {
		t.Fatalf("challenge cleanup requests = %v", queries)
	}
}

func TestIssueRejectsACMEAdapterFailures(t *testing.T) {
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	duckDNS := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("OK")) }))
	defer duckDNS.Close()
	for _, failure := range []string{"register", "order", "authorization", "challenge", "record", "wait-authorization", "wait-order", "certificate", "empty-chain"} {
		t.Run(failure, func(t *testing.T) {
			client := newFakeACME(t, time.Now())
			client.fail = failure
			provider := duckDNSProvider{config: config, client: duckDNS.Client(), endpoint: duckDNS.URL}
			if _, err := issue(t.Context(), config, client, provider, func(context.Context, string, string) error { return nil }); err == nil {
				t.Fatalf("%s failure was accepted", failure)
			}
		})
	}
}

func TestIssuerHelpersRejectInvalidBoundaries(t *testing.T) { //nolint:cyclop,gocognit // Independent negative adapter boundaries share one compact table.
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	for name, handler := range map[string]http.Handler{
		"status": http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { http.Error(writer, "no", http.StatusBadGateway) }),
		"body":   http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("unexpected")) }),
		"large":  http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write(make([]byte, 65)) }),
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewTLSServer(handler)
			defer server.Close()
			if err := updateDuckDNS(t.Context(), server.Client(), server.URL, config, "", false); err == nil {
				t.Fatal("invalid DuckDNS response was accepted")
			}
		})
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if waitContext(canceled, time.Hour) || !waitContext(t.Context(), time.Millisecond) {
		t.Fatal("context wait did not honor cancellation and timer completion")
	}
	identity := testIdentity(t, config.Hostname(), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificate, err := tls.X509KeyPair(identity, identity)
	if err != nil {
		t.Fatal(err)
	}
	leaf := certificate.Certificate[0]
	wrongKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = encodeIdentityWith([][]byte{leaf}, wrongKey, config.Hostname(), time.Now(), defaultIdentityOperations()); err == nil {
		t.Fatal("mismatched certificate key was accepted")
	}
	if _, err = encodeIdentityWith([][]byte{leaf}, wrongKey, "other.example", time.Now(), defaultIdentityOperations()); err == nil {
		t.Fatal("wrong certificate hostname was accepted")
	}
}

type fakeACME struct {
	t        *testing.T
	now      time.Time
	signer   *ecdsa.PrivateKey
	fail     string
	accepted bool
}

var _ acmeClient = (*fakeACME)(nil)

func newFakeACME(t *testing.T, now time.Time) *fakeACME {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeACME{t: t, now: now, signer: key}
}

func (client *fakeACME) Register(_ context.Context, _ *acme.Account, prompt func(string) bool) (*acme.Account, error) {
	if client.fail == "account-exists" {
		return nil, acme.ErrAccountAlreadyExists
	}
	if client.fail == "register" {
		return nil, errors.New("register")
	}
	if !prompt("terms") {
		client.t.Fatal("terms prompt was rejected")
	}
	return &acme.Account{Status: acme.StatusValid}, nil
}

func (client *fakeACME) AuthorizeOrder(context.Context, []acme.AuthzID, ...acme.OrderOption) (*acme.Order, error) {
	if client.fail == "order" {
		return nil, errors.New("order")
	}
	if client.fail == "nil-order" {
		return nil, nil
	}
	if client.fail == "many-authorizations" {
		return &acme.Order{URI: "order", AuthzURLs: []string{"one", "two"}, FinalizeURL: "finalize"}, nil
	}
	if client.fail == "missing-finalize" {
		return &acme.Order{URI: "order", AuthzURLs: []string{"authorization"}}, nil
	}
	if client.fail == "missing-order-uri" {
		return &acme.Order{AuthzURLs: []string{"authorization"}, FinalizeURL: "finalize"}, nil
	}
	return &acme.Order{URI: "order", AuthzURLs: []string{"authorization"}, FinalizeURL: "finalize"}, nil
}

func (client *fakeACME) GetAuthorization(context.Context, string) (*acme.Authorization, error) {
	if client.fail == "authorization" {
		return nil, errors.New("authorization")
	}
	if client.fail == "challenge" {
		return &acme.Authorization{URI: "authorization", Status: acme.StatusPending, Challenges: []*acme.Challenge{{Type: "http-01"}}}, nil
	}
	if client.fail == "nil-authorization" {
		return nil, nil
	}
	if client.fail == "too-many-challenges" {
		return &acme.Authorization{Challenges: make([]*acme.Challenge, 33)}, nil
	}
	if client.fail == "valid-authorization" {
		return &acme.Authorization{Status: acme.StatusValid}, nil
	}
	return &acme.Authorization{URI: "authorization", Status: acme.StatusPending, Challenges: []*acme.Challenge{{Type: "dns-01", URI: "challenge", Token: "token"}}}, nil
}

func (client *fakeACME) DNS01ChallengeRecord(string) (string, error) {
	if client.fail == "record" {
		return "", errors.New("record")
	}
	if client.fail == "empty-record" {
		return "", nil
	}
	if client.fail == "large-record" {
		return string(make([]byte, 513)), nil
	}
	return "dns-proof", nil
}

func (client *fakeACME) Accept(_ context.Context, challenge *acme.Challenge) (*acme.Challenge, error) {
	if client.fail == "accept" {
		return nil, errors.New("accept")
	}
	client.accepted = true
	return challenge, nil
}

func (client *fakeACME) WaitAuthorization(context.Context, string) (*acme.Authorization, error) {
	if client.fail == "wait-authorization" {
		return nil, errors.New("authorization")
	}
	if client.fail == "nil-completed-authorization" {
		return nil, nil
	}
	if client.fail == "invalid-completed-authorization" {
		return &acme.Authorization{Status: acme.StatusInvalid}, nil
	}
	return &acme.Authorization{Status: acme.StatusValid}, nil
}

func (client *fakeACME) WaitOrder(context.Context, string) (*acme.Order, error) {
	if client.fail == "wait-order" {
		return nil, errors.New("order")
	}
	if client.fail == "nil-wait-order" {
		return nil, nil
	}
	if client.fail == "invalid-wait-order" {
		return &acme.Order{Status: acme.StatusInvalid, FinalizeURL: "finalize"}, nil
	}
	if client.fail == "wait-order-no-finalize" {
		return &acme.Order{Status: acme.StatusReady}, nil
	}
	return &acme.Order{Status: acme.StatusReady, FinalizeURL: "finalize"}, nil
}

func (client *fakeACME) CreateOrderCert(_ context.Context, _ string, csrDER []byte, _ bool) ([][]byte, string, error) {
	if client.fail == "certificate" {
		return nil, "", errors.New("certificate")
	}
	if client.fail == "empty-chain" {
		return nil, "", nil
	}
	if client.fail == "large-chain" {
		return make([][]byte, 11), "", nil
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, "", err
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: csr.DNSNames[0]}, DNSNames: csr.DNSNames, NotBefore: client.now.Add(-time.Hour), NotAfter: client.now.Add(90 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, csr.PublicKey, client.signer)
	return [][]byte{der}, "", err
}
