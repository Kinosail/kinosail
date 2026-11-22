package updatecontrol

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/sigstore/sigstore-go/pkg/root"
)

const (
	ChannelSchemaVersion = 1
	ReleaseDelay         = 24 * time.Hour
	maximumRevocations   = 100
	channelLifetime      = 8 * 24 * time.Hour
	channelSigningWindow = time.Hour
	clockSkew            = 5 * time.Minute
)

var errInvalidChannel = errors.New("invalid update channel")

type Channel struct {
	SchemaVersion int       `json:"schemaVersion"`
	App           string    `json:"app"`
	Latest        string    `json:"latest"`
	Revoked       []string  `json:"revoked"`
	Issued        time.Time `json:"issued"`
	Expires       time.Time `json:"expires"`
}

type UpdateDecision struct {
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	Version   string `json:"version,omitempty"`
	NotBefore string `json:"notBefore,omitempty"`
}

func VerifyChannel(material root.TrustedMaterial, policy Policy, statement, signature []byte) (Channel, error) {
	if validatePolicy(policy) != nil {
		return Channel{}, errInvalidChannel
	}
	evidence, err := verifySignedBlob(material, channelProvenance(), statement, signature)
	if err != nil {
		return Channel{}, err
	}
	return checkChannel(statement, evidence, policy)
}

func checkChannel(statement []byte, evidence SignatureEvidence, policy Policy) (Channel, error) {
	var channel Channel
	if httpguard.DecodeUniqueJSON(bytes.NewReader(statement), signedInputLimit, &channel) != nil || channel.SchemaVersion != ChannelSchemaVersion ||
		channel.App != policy.tagPrefix || !validReleaseVersion(channel.Latest) || len(channel.Revoked) > maximumRevocations {
		return Channel{}, errInvalidChannel
	}
	for index, version := range channel.Revoked {
		if !validReleaseVersion(version) || version == channel.Latest || slices.Contains(channel.Revoked[:index], version) {
			return Channel{}, errInvalidChannel
		}
	}
	lifetime := channel.Expires.Sub(channel.Issued)
	signedAfterIssue := evidence.SignedAt.Sub(channel.Issued)
	if channel.Issued.IsZero() || lifetime <= 0 || lifetime > channelLifetime || signedAfterIssue < -clockSkew || signedAfterIssue > channelSigningWindow {
		return Channel{}, errInvalidChannel
	}
	return channel, nil
}

func DecideUpdate(installed string, channel Channel, release SignatureEvidence, now time.Time) UpdateDecision {
	parts, managed := releaseVersion(installed)
	if !managed {
		return UpdateDecision{Action: "none", Reason: "unmanaged-version"}
	}
	if now.After(channel.Expires) || channel.Issued.After(now.Add(clockSkew)) {
		return UpdateDecision{Action: "none", Reason: "channel-stale"}
	}
	if release.SignedAt.After(now.Add(clockSkew)) {
		return UpdateDecision{Action: "none", Reason: "release-time"}
	}
	if !newerRelease(installed, channel.Latest) {
		return UpdateDecision{Action: "none", Reason: "current"}
	}
	if slices.Contains(channel.Revoked, fmt.Sprintf("v%d.%d.%d", parts[0], parts[1], parts[2])) {
		return UpdateDecision{Action: "install", Reason: "installed-revoked", Version: channel.Latest}
	}
	if eligible := release.SignedAt.Add(ReleaseDelay); now.Before(eligible) {
		return UpdateDecision{Action: "wait", Reason: "release-delay", Version: channel.Latest, NotBefore: eligible.UTC().Format(time.RFC3339)}
	}
	return UpdateDecision{Action: "install", Reason: "newer-release", Version: channel.Latest}
}
