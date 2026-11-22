package updatecontrol

import (
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

type certificateExtensions = certificate.Extensions

var channelSigned = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

func pinnedExtensions(expected provenance) certificate.Extensions {
	return certificate.Extensions{
		Issuer:                 actionsIssuer,
		BuildSignerURI:         expected.identity(),
		BuildConfigURI:         expected.identity(),
		RunnerEnvironment:      "github-hosted",
		SourceRepositoryURI:    sourceRepository,
		SourceRepositoryDigest: "14f761e2de9b47133a235a0fef453fde504d768b",
		SourceRepositoryRef:    expected.ref,
		BuildTrigger:           expected.triggers[0],
	}
}

func provenanceResult(extensions certificate.Extensions) *verify.VerificationResult {
	return &verify.VerificationResult{
		Signature:          &verify.SignatureVerificationResult{Certificate: &certificate.Summary{Extensions: extensions}},
		VerifiedTimestamps: []verify.TimestampVerificationResult{{Type: "Tlog", Timestamp: channelSigned.Add(time.Minute)}, {Type: "TimestampAuthority", Timestamp: channelSigned}},
	}
}

func channelStatement(fields ...string) []byte {
	values := map[string]string{
		"schemaVersion": `1`, "app": `"player"`, "latest": `"v0.2.0"`, "revoked": `["v0.1.3"]`,
		"issued": `"2026-11-22T11:58:00Z"`, "expires": `"2026-11-30T11:58:00Z"`,
	}
	for index := 0; index+1 < len(fields); index += 2 {
		values[fields[index]] = fields[index+1]
	}
	parts := []string{}
	for _, key := range []string{"schemaVersion", "app", "latest", "revoked", "issued", "expires", "extra"} {
		if value, ok := values[key]; ok && value != "-" {
			parts = append(parts, `"`+key+`":`+value)
		}
	}
	return []byte("{" + strings.Join(parts, ",") + "}")
}

func TestChannelStatementAcceptsOnlyAConsistentFreshAnnouncement(t *testing.T) {
	t.Parallel()
	evidence := SignatureEvidence{SignedAt: channelSigned, Commit: strings.Repeat("a", 40)}
	channel, err := checkChannel(channelStatement(), evidence, PlayerPolicy(1, 1))
	if err != nil || channel.Latest != "v0.2.0" || len(channel.Revoked) != 1 || !channel.Expires.Equal(time.Date(2026, 11, 30, 11, 58, 0, 0, time.UTC)) {
		t.Fatalf("valid channel = %+v, %v", channel, err)
	}
	if _, err := checkChannel(channelStatement(), evidence, SubtitlesPolicy(1, 1)); err == nil {
		t.Error("player channel accepted for Subtitles")
	}
	rejected := map[string][]byte{
		"schema":             channelStatement("schemaVersion", `2`),
		"app":                channelStatement("app", `"subtitles"`),
		"latest":             channelStatement("latest", `"0.2.0"`),
		"missing latest":     channelStatement("latest", "-"),
		"latest revoked":     channelStatement("revoked", `["v0.2.0"]`),
		"duplicate revoked":  channelStatement("revoked", `["v0.1.3","v0.1.3"]`),
		"invalid revoked":    channelStatement("revoked", `["latest"]`),
		"too many revoked":   channelStatement("revoked", `[`+strings.TrimSuffix(strings.Repeat(`"v0.0.1",`, maximumRevocations+1), ",")+`]`),
		"unknown field":      channelStatement("extra", `true`),
		"expires early":      channelStatement("expires", `"2026-11-22T11:00:00Z"`),
		"long lifetime":      channelStatement("expires", `"2026-12-22T11:58:00Z"`),
		"issued after sign":  channelStatement("issued", `"2026-11-22T12:30:00Z"`, "expires", `"2026-11-29T12:30:00Z"`),
		"issued long before": channelStatement("issued", `"2026-11-22T09:00:00Z"`, "expires", `"2026-11-29T09:00:00Z"`),
		"missing issued":     channelStatement("issued", "-"),
		"not json":           []byte("latest=v0.2.0"),
	}
	for name, statement := range rejected {
		if _, err := checkChannel(statement, evidence, PlayerPolicy(1, 1)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestUpdateDecisionHonorsDelayRevocationRollbackAndFreshness(t *testing.T) {
	t.Parallel()
	channel := Channel{SchemaVersion: 1, App: "player", Latest: "v0.2.0", Revoked: []string{"v0.1.3"}, Issued: channelSigned, Expires: channelSigned.Add(7 * 24 * time.Hour)}
	released := SignatureEvidence{SignedAt: channelSigned.Add(-30 * time.Hour), Commit: strings.Repeat("b", 40)}
	recent := SignatureEvidence{SignedAt: channelSigned.Add(-2 * time.Hour), Commit: released.Commit}
	now := channelSigned.Add(time.Hour)
	cases := []struct {
		name      string
		installed string
		channel   Channel
		release   SignatureEvidence
		now       time.Time
		action    string
		reason    string
	}{
		{"older version after the delay", "v0.1.9", channel, released, now, "install", "newer-release"},
		{"recent release waits", "v0.1.9", channel, recent, now, "wait", "release-delay"},
		{"revoked installed version skips the delay", "v0.1.3", channel, recent, now, "install", "installed-revoked"},
		{"same version", "v0.2.0", channel, released, now, "none", "current"},
		{"newer installed version never downgrades", "v0.3.0", channel, released, now, "none", "current"},
		{"expired channel", "v0.1.9", channel, released, channel.Expires.Add(time.Second), "none", "channel-stale"},
		{"channel from the future", "v0.1.9", channel, released, channel.Issued.Add(-time.Hour), "none", "channel-stale"},
		{"development build", "dev", channel, released, now, "none", "unmanaged-version"},
		{"release signed in the future", "v0.1.9", channel, SignatureEvidence{SignedAt: now.Add(time.Hour)}, now, "none", "release-time"},
	}
	for _, test := range cases {
		decision := DecideUpdate(test.installed, test.channel, test.release, test.now)
		if decision.Action != test.action || decision.Reason != test.reason {
			t.Errorf("%s = %+v", test.name, decision)
		}
		if decision.Action == "install" && decision.Version != "v0.2.0" {
			t.Errorf("%s installs %q", test.name, decision.Version)
		}
	}
	waiting := DecideUpdate("v0.1.9", channel, recent, now)
	if waiting.NotBefore != recent.SignedAt.Add(ReleaseDelay).Format(time.RFC3339) {
		t.Errorf("wait deadline = %q", waiting.NotBefore)
	}
}
