package updatecontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
)

const (
	CommandVerify       = "update-verify"
	VerifySchemaVersion = 1
	verifyTimeout       = 60 * time.Second
)

type TrustConfig struct {
	Client   HTTPClient
	Now      func() time.Time
	Material func(string) (root.TrustedMaterial, error)
}

type VerifiedUpdate struct {
	SchemaVersion int `json:"schemaVersion"`
	UpdateDecision
	Commit   string    `json:"commit"`
	SignedAt string    `json:"signedAt"`
	Artifact *Artifact `json:"artifact,omitempty"`
}

func defaultTrust(config TrustConfig) TrustConfig {
	if config.Client == nil {
		config.Client = &http.Client{Timeout: verifyTimeout}
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Material == nil {
		config.Material = publicGoodTrustRoot
	}
	return config
}

func publicGoodTrustRoot(dataDir string) (root.TrustedMaterial, error) {
	return root.FetchTrustedRootWithOptions(tuf.DefaultOptions().WithCachePath(filepath.Join(dataDir, "sigstore")))
}

func channelURL(app string) string {
	return fmt.Sprintf("https://github.com/Kinosail/kinosail/releases/download/%s-channel/channel.json", app)
}

func (commands commandSet) verifyUpdate(output io.Writer, dataDir, goos, arch string) error {
	if output == nil {
		return errors.New("update output is unavailable")
	}
	if dataDir == "" {
		return errors.New("update storage is unavailable")
	}
	trust, policy := commands.config.Trust, commands.config.Policy
	material, err := trust.Material(dataDir)
	if err != nil || material == nil {
		return errors.New("the Sigstore trust root is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
	defer cancel()
	statement, signature, err := fetchSignedPair(ctx, trust.Client, channelURL(policy.tagPrefix), channelURL(policy.tagPrefix)+".sigstore.json")
	if err != nil {
		return err
	}
	channel, err := VerifyChannel(material, policy, statement, signature)
	if err != nil {
		return err
	}
	manifestURL, manifestSignatureURL := releaseManifestURLs(channel.Latest, policy)
	manifestData, manifestSignature, err := fetchSignedPair(ctx, trust.Client, manifestURL, manifestSignatureURL)
	if err != nil {
		return err
	}
	evidence, err := VerifyReleaseSignature(material, policy, channel.Latest, manifestData, manifestSignature)
	if err != nil {
		return err
	}
	manifest, err := parseManifest(bytes.NewReader(manifestData))
	if err != nil || manifest.Version != channel.Latest {
		return errors.New("release manifest does not match the update channel")
	}
	result := VerifiedUpdate{SchemaVersion: VerifySchemaVersion, UpdateDecision: DecideUpdate(commands.config.Version(), channel, evidence, trust.Now()), Commit: evidence.Commit, SignedAt: evidence.SignedAt.Format(time.RFC3339)}
	if result.Action == "install" {
		if err = manifest.Compatible(policy.stateSchema, policy.configurationSchema); err != nil {
			return err
		}
		artifact, err := manifest.Select(goos, arch)
		if err != nil {
			return err
		}
		result.Artifact = &artifact
	}
	return json.NewEncoder(output).Encode(result)
}

func fetchSignedPair(ctx context.Context, client HTTPClient, contentURL, signatureURL string) ([]byte, []byte, error) {
	content, err := fetchSigned(ctx, client, contentURL)
	if err != nil {
		return nil, nil, err
	}
	signature, err := fetchSigned(ctx, client, signatureURL)
	return content, signature, err
}

func fetchSigned(ctx context.Context, client HTTPClient, target string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil || target == "" {
		return nil, errors.New("invalid update download address")
	}
	request.Header.Set("User-Agent", "Kinosail-update-check")
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("GitHub could not be reached")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, signedInputLimit+1))
	if response.StatusCode != http.StatusOK || err != nil || len(data) == 0 || len(data) > signedInputLimit {
		return nil, errors.New("update download failed")
	}
	return data, nil
}
