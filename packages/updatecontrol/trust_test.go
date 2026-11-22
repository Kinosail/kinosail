package updatecontrol

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
)

func publicGoodRoot(t *testing.T) root.TrustedMaterial {
	t.Helper()
	material, err := root.NewTrustedRootFromJSON(readFixture(t, "sigstore-trusted-root.json"))
	if err != nil {
		t.Fatal(err)
	}
	return material
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPublishedReleaseSignaturesVerifyWithPinnedProvenance(t *testing.T) {
	t.Parallel()
	material := publicGoodRoot(t)
	for _, file := range []string{"kinosail-release.json", "SHA256SUMS"} {
		evidence, err := VerifyReleaseSignature(material, PlayerPolicy(1, 1), "v0.1.0", readFixture(t, "player-v0.1.0/"+file), readFixture(t, "player-v0.1.0/"+file+".sigstore.json"))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if evidence.Commit != "14f761e2de9b47133a235a0fef453fde504d768b" || evidence.SignedAt.Year() != 2026 || evidence.SignedAt.After(time.Unix(1795384815, 0)) {
			t.Fatalf("%s evidence = %+v", file, evidence)
		}
	}
}

func TestReleaseSignatureRejectsReplayTamperingAndForeignIdentities(t *testing.T) {
	t.Parallel()
	material := publicGoodRoot(t)
	manifest := readFixture(t, "player-v0.1.0/kinosail-release.json")
	signature := readFixture(t, "player-v0.1.0/kinosail-release.json.sigstore.json")
	sumsSignature := readFixture(t, "player-v0.1.0/SHA256SUMS.sigstore.json")
	virtual, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		material  root.TrustedMaterial
		policy    Policy
		version   string
		artifact  []byte
		signature []byte
	}{
		"newer version claimed for an old signature": {material, PlayerPolicy(1, 1), "v0.1.1", manifest, signature},
		"older version claimed":                      {material, PlayerPolicy(1, 1), "v0.0.9", manifest, signature},
		"another app":                                {material, SubtitlesPolicy(1, 1), "v0.1.0", manifest, signature},
		"tampered artifact":                          {material, PlayerPolicy(1, 1), "v0.1.0", bytes.Replace(manifest, []byte("v0.1.0"), []byte("v0.1.1"), 1), signature},
		"signature for another file":                 {material, PlayerPolicy(1, 1), "v0.1.0", manifest, sumsSignature},
		"no transparency log entry":                  {material, PlayerPolicy(1, 1), "v0.1.0", manifest, withoutTransparencyLog(t, signature)},
		"untrusted certificate authority":            {virtual, PlayerPolicy(1, 1), "v0.1.0", manifest, signature},
		"malformed signature":                        {material, PlayerPolicy(1, 1), "v0.1.0", manifest, []byte("{")},
		"oversized signature":                        {material, PlayerPolicy(1, 1), "v0.1.0", manifest, bytes.Repeat([]byte(" "), signedInputLimit+1)},
		"oversized artifact":                         {material, PlayerPolicy(1, 1), "v0.1.0", bytes.Repeat([]byte(" "), signedInputLimit+1), signature},
		"empty artifact":                             {material, PlayerPolicy(1, 1), "v0.1.0", nil, signature},
		"invalid version":                            {material, PlayerPolicy(1, 1), "0.1.0", manifest, signature},
		"no trusted root":                            {nil, PlayerPolicy(1, 1), "v0.1.0", manifest, signature},
		"invalid policy":                             {material, Policy{}, "v0.1.0", manifest, signature},
	}
	for name, test := range cases {
		if _, err := VerifyReleaseSignature(test.material, test.policy, test.version, test.artifact, test.signature); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestSignatureProvenanceRejectsForksOtherWorkflowsAndBranches(t *testing.T) {
	t.Parallel()
	virtual, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte(`{"app":"player"}`)
	for _, expected := range []provenance{releaseProvenance("player", "v0.1.0"), channelProvenance()} {
		entity, err := virtual.Sign(expected.identity(), actionsIssuer, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := verifyEntity(virtual, expected.identity(), artifact, entity, false); err != nil {
			t.Fatalf("matching %s rejected: %v", expected.identity(), err)
		}
		if _, err := verifyEntity(virtual, expected.identity(), []byte(`{"app":"other"}`), entity, false); err == nil {
			t.Fatalf("changed artifact accepted for %s", expected.identity())
		}
	}
	identities := []string{
		"https://github.com/attacker/kinosail/.github/workflows/release.yml@refs/tags/player-v0.1.0",
		"https://github.com/Kinosail/kinosail/.github/workflows/ci.yml@refs/tags/player-v0.1.0",
		"https://github.com/Kinosail/kinosail/.github/workflows/release.yml@refs/heads/main",
		"https://github.com/Kinosail/kinosail/.github/workflows/release-trust.yml@refs/heads/feature",
	}
	for _, identity := range identities {
		entity, err := virtual.Sign(identity, actionsIssuer, artifact)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []provenance{releaseProvenance("player", "v0.1.0"), channelProvenance()} {
			if _, err := verifyEntity(virtual, expected.identity(), artifact, entity, false); err == nil {
				t.Errorf("%s accepted for %s", identity, expected.identity())
			}
		}
	}
	entity, err := virtual.Sign(channelProvenance().identity(), "https://accounts.example", artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyEntity(virtual, channelProvenance().identity(), artifact, entity, false); err == nil {
		t.Error("foreign OIDC issuer accepted")
	}
}

func TestProvenanceRequiresEveryPinnedCertificateExtension(t *testing.T) {
	t.Parallel()
	expected := releaseProvenance("player", "v0.1.0")
	valid := pinnedExtensions(expected)
	if _, err := expected.evidence(provenanceResult(valid)); err != nil {
		t.Fatalf("valid extensions rejected: %v", err)
	}
	mutations := map[string]func(*certificateExtensions){
		"issuer":     func(e *certificateExtensions) { e.Issuer = "https://accounts.example" },
		"repository": func(e *certificateExtensions) { e.SourceRepositoryURI = "https://github.com/attacker/kinosail" },
		"ref":        func(e *certificateExtensions) { e.SourceRepositoryRef = "refs/heads/main" },
		"signer": func(e *certificateExtensions) {
			e.BuildSignerURI = strings.Replace(e.BuildSignerURI, "release.yml", "ci.yml", 1)
		},
		"config":         func(e *certificateExtensions) { e.BuildConfigURI = "" },
		"runner":         func(e *certificateExtensions) { e.RunnerEnvironment = "self-hosted" },
		"trigger":        func(e *certificateExtensions) { e.BuildTrigger = "pull_request_target" },
		"short commit":   func(e *certificateExtensions) { e.SourceRepositoryDigest = "14f761e2" },
		"invalid commit": func(e *certificateExtensions) { e.SourceRepositoryDigest = strings.Repeat("Z", 40) },
	}
	for name, mutate := range mutations {
		extensions := valid
		mutate(&extensions)
		if _, err := expected.evidence(provenanceResult(extensions)); err == nil {
			t.Errorf("%s mismatch accepted", name)
		}
	}
	if _, err := expected.evidence(nil); err == nil {
		t.Error("missing verification result accepted")
	}
	untimed := provenanceResult(valid)
	untimed.VerifiedTimestamps = nil
	if _, err := expected.evidence(untimed); err == nil {
		t.Error("result without a verified timestamp accepted")
	}
}

func withoutTransparencyLog(t *testing.T, signature []byte) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(signature, &document); err != nil {
		t.Fatal(err)
	}
	material := document["verificationMaterial"].(map[string]any)
	delete(material, "tlogEntries")
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
