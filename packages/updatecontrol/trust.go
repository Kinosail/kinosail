package updatecontrol

import (
	"bytes"
	"errors"
	"slices"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const (
	sourceRepository = "https://github.com/Kinosail/kinosail"
	actionsIssuer    = "https://token.actions.githubusercontent.com"
	signedInputLimit = 64 << 10
)

var errUntrustedSignature = errors.New("release signature is not trusted")

type SignatureEvidence struct {
	SignedAt time.Time
	Commit   string
}

type provenance struct {
	workflow string
	ref      string
	triggers []string
}

func releaseProvenance(app, version string) provenance {
	return provenance{workflow: ".github/workflows/release.yml", ref: "refs/tags/" + app + "-" + version, triggers: []string{"push"}}
}

func channelProvenance() provenance {
	return provenance{workflow: ".github/workflows/release-trust.yml", ref: "refs/heads/main", triggers: []string{"schedule", "workflow_dispatch"}}
}

func (expected provenance) identity() string {
	return sourceRepository + "/" + expected.workflow + "@" + expected.ref
}

func VerifyReleaseSignature(material root.TrustedMaterial, policy Policy, version string, artifact, signature []byte) (SignatureEvidence, error) {
	if validatePolicy(policy) != nil || !validReleaseVersion(version) {
		return SignatureEvidence{}, errUntrustedSignature
	}
	return verifySignedBlob(material, releaseProvenance(policy.tagPrefix, version), artifact, signature)
}

func verifySignedBlob(material root.TrustedMaterial, expected provenance, artifact, signature []byte) (SignatureEvidence, error) {
	if len(signature) == 0 || len(signature) > signedInputLimit {
		return SignatureEvidence{}, errUntrustedSignature
	}
	var entity bundle.Bundle
	if entity.UnmarshalJSON(signature) != nil {
		return SignatureEvidence{}, errUntrustedSignature
	}
	result, err := verifyEntity(material, expected.identity(), artifact, &entity, true)
	if err != nil {
		return SignatureEvidence{}, err
	}
	return expected.evidence(result)
}

func verifyEntity(material root.TrustedMaterial, identity string, artifact []byte, entity verify.SignedEntity, certificateTransparency bool) (*verify.VerificationResult, error) {
	if material == nil || entity == nil || len(artifact) == 0 || len(artifact) > signedInputLimit {
		return nil, errUntrustedSignature
	}
	options := []verify.VerifierOption{verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1)}
	if certificateTransparency {
		options = append(options, verify.WithSignedCertificateTimestamps(1))
	}
	verifier, err := verify.NewVerifier(material, options...)
	if err != nil {
		return nil, errUntrustedSignature
	}
	certificateIdentity, err := verify.NewShortCertificateIdentity(actionsIssuer, "", identity, "")
	if err != nil {
		return nil, errUntrustedSignature
	}
	result, err := verifier.Verify(entity, verify.NewPolicy(verify.WithArtifact(bytes.NewReader(artifact)), verify.WithCertificateIdentity(certificateIdentity)))
	if err != nil {
		return nil, errUntrustedSignature
	}
	return result, nil
}

func (expected provenance) evidence(result *verify.VerificationResult) (SignatureEvidence, error) {
	if result == nil || result.Signature == nil || result.Signature.Certificate == nil || len(result.VerifiedTimestamps) == 0 {
		return SignatureEvidence{}, errUntrustedSignature
	}
	extensions := result.Signature.Certificate.Extensions
	identity := expected.identity()
	if extensions.Issuer != actionsIssuer || extensions.SourceRepositoryURI != sourceRepository || extensions.SourceRepositoryRef != expected.ref ||
		extensions.BuildSignerURI != identity || extensions.BuildConfigURI != identity || extensions.RunnerEnvironment != "github-hosted" ||
		!slices.Contains(expected.triggers, extensions.BuildTrigger) || len(extensions.SourceRepositoryDigest) != 40 || !lowerHex(extensions.SourceRepositoryDigest) {
		return SignatureEvidence{}, errUntrustedSignature
	}
	signed := result.VerifiedTimestamps[0].Timestamp
	for _, stamp := range result.VerifiedTimestamps[1:] {
		if stamp.Timestamp.Before(signed) {
			signed = stamp.Timestamp
		}
	}
	return SignatureEvidence{SignedAt: signed.UTC(), Commit: extensions.SourceRepositoryDigest}, nil
}
