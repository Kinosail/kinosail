package trustedhttps

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddressResolutionCoversDirectResolverAndEmptyResults(t *testing.T) {
	for _, address := range []string{"203.0.113.1", "fd00::1"} {
		if _, err := (Config{Address: address}).withResolvedAddress(t.Context(), nil); err == nil {
			t.Fatalf("unsafe direct address %q accepted", address)
		}
	}
	if _, err := (Config{Address: "localhost"}).withResolvedAddress(t.Context(), nil); err == nil {
		t.Fatal("loopback hostname accepted")
	}
	for name, lookup := range map[string]func(context.Context, string) ([]net.IP, error){
		"error": func(context.Context, string) ([]net.IP, error) { return nil, errors.New("offline") },
		"empty": func(context.Context, string) ([]net.IP, error) { return nil, nil },
		"non IPv4 then private": func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("192.168.1.20")}, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			config, err := (Config{Address: "server.nox"}).withResolvedAddress(t.Context(), lookup)
			if name == "non IPv4 then private" {
				if err != nil || config.Address != "192.168.1.20" {
					t.Fatalf("resolved config = %#v, %v", config, err)
				}
			} else if err == nil {
				t.Fatal("unsafe resolution accepted")
			}
		})
	}
}

func TestIssueCoversValidAuthorizationAndMalformedOrders(t *testing.T) {
	config := validIssuerConfig()
	for _, failure := range []string{"account-exists", "nil-order", "many-authorizations", "missing-finalize", "missing-order-uri", "nil-authorization", "too-many-challenges", "empty-record", "large-record", "nil-completed-authorization", "invalid-completed-authorization", "nil-wait-order", "invalid-wait-order", "wait-order-no-finalize", "large-chain"} {
		t.Run(failure, func(t *testing.T) {
			client := newFakeACME(t, time.Now())
			client.fail = failure
			provider := &issuerProvider{}
			_, err := issue(t.Context(), config, client, provider, func(context.Context, string, string) error { return nil })
			if failure == "account-exists" || failure == "valid-authorization" {
				if err != nil {
					t.Fatalf("accepted ACME state = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ACME failure %q accepted", failure)
			}
		})
	}
	client := newFakeACME(t, time.Now())
	client.fail = "valid-authorization"
	if _, err := issue(t.Context(), config, client, &issuerProvider{}, func(context.Context, string, string) error { return nil }); err != nil {
		t.Fatalf("valid existing authorization = %v", err)
	}
}

func TestAuthorizationAlwaysCleansUpPublishedProof(t *testing.T) {
	config := validIssuerConfig()
	for name, setup := range map[string]func(*fakeACME, *issuerProvider) func(context.Context, string, string) error{
		"present": func(_ *fakeACME, provider *issuerProvider) func(context.Context, string, string) error {
			provider.presentErr = errors.New("present failed")
			return func(context.Context, string, string) error { return nil }
		},
		"propagation": func(_ *fakeACME, _ *issuerProvider) func(context.Context, string, string) error {
			return func(context.Context, string, string) error { return errors.New("not propagated") }
		},
		"cleanup": func(_ *fakeACME, provider *issuerProvider) func(context.Context, string, string) error {
			provider.cleanupErr = errors.New("cleanup failed")
			return func(context.Context, string, string) error { return nil }
		},
	} {
		t.Run(name, func(t *testing.T) {
			client, provider := newFakeACME(t, time.Now()), &issuerProvider{}
			wait := setup(client, provider)
			err := authorizeDNS(t.Context(), config, client, provider, wait, "authorization")
			if err == nil {
				t.Fatal("authorization failure accepted")
			}
			if name == "propagation" && provider.cleanupCalls != 1 {
				t.Fatalf("cleanup calls = %d", provider.cleanupCalls)
			}
		})
	}
}

func TestDuckDNSRequestAndResponseFailures(t *testing.T) {
	config := validIssuerConfig()
	if err := updateDuckDNS(t.Context(), http.DefaultClient, "%", config, "", false); err == nil {
		t.Fatal("invalid update address accepted")
	}
	for name, client := range map[string]*http.Client{
		"network": {Transport: providerTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })},
		"body": {Transport: providerTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: providerFailingBody{}}, nil
		})},
	} {
		t.Run(name, func(t *testing.T) {
			if err := updateDuckDNS(t.Context(), client, duckDNSUpdateURL, config, "", false); err == nil {
				t.Fatal("failed update accepted")
			}
		})
	}
}

func TestAccountKeyRejectsUnreadableAndWrongKeyTypes(t *testing.T) {
	directory := t.TempDir()
	if _, err := accountKey(directory); err == nil {
		t.Fatal("directory account key accepted")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "public.pem")
	if err = os.WriteFile(path, []byte("-----BEGIN PRIVATE KEY-----\n"+strings.TrimSpace(string(public))+"\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = accountKey(path); err == nil {
		t.Fatal("non-private key accepted")
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaDER, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: rsaDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = accountKey(path); err == nil {
		t.Fatal("non-ECDSA private key accepted")
	}
}

func validIssuerConfig() Config {
	return Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
}

type issuerProvider struct {
	presentErr   error
	cleanupErr   error
	cleanupCalls int
}

func (*issuerProvider) UpdateAddress(context.Context) error { return nil }

func (provider *issuerProvider) PresentTXT(context.Context, string) error {
	return provider.presentErr
}

func (provider *issuerProvider) CleanupTXT(context.Context) error {
	provider.cleanupCalls++
	return provider.cleanupErr
}

func TestObtainCertificateUsesDefaultDirectory(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	path := t.TempDir()
	if _, err := obtainCertificate(t.Context(), validIssuerConfig(), path, server.Client(), server.URL, &issuerProvider{}); err == nil {
		t.Fatal("invalid ACME directory accepted")
	}
	if _, err := obtainCertificate(t.Context(), validIssuerConfig(), t.TempDir(), &http.Client{Transport: failingTransport{}}, "", &issuerProvider{}); err == nil {
		t.Fatal("unavailable default ACME directory accepted")
	}
}

func TestTXTWaitUsesBoundedRetryPolicy(t *testing.T) {
	calls := 0
	err := waitForTXTWith(t.Context(), "_acme.example", "proof", func(context.Context, string) ([]string, error) {
		calls++
		if calls == 1 {
			return []string{"other"}, nil
		}
		return []string{"proof"}, nil
	}, func(context.Context, time.Duration) bool { return true })
	if err != nil || calls != 2 {
		t.Fatalf("TXT propagation = %v, calls=%d", err, calls)
	}
	if err = waitForTXTWith(t.Context(), "_acme.example", "proof", func(context.Context, string) ([]string, error) {
		return nil, errors.New("offline")
	}, func(context.Context, time.Duration) bool { return false }); err == nil {
		t.Fatal("failed TXT propagation accepted")
	}
	err = waitForTXTWith(t.Context(), "_acme.example", "proof", func(ctx context.Context, _ string) ([]string, error) {
		deadline, ok := ctx.Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining < 5*time.Minute-time.Second || remaining > 5*time.Minute {
			t.Fatalf("TXT deadline = %v, present=%t", remaining, ok)
		}
		return []string{"proof"}, nil
	}, func(context.Context, time.Duration) bool { return false })
	if err != nil {
		t.Fatalf("deadline check = %v", err)
	}
}
