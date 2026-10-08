package server

import (
	"strings"
	"testing"
)

// Preserve the leading16 samples and check the phase transition independently.
func TestRemainingNonKeySourceAudioPartialBootstrapDamage(t *testing.T) {
	cases := []struct {
		name   string
		damage func(*remainingNonKeySourceAudioFixture)
	}{
		{"leading-samples", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[0]["nb_samples"] = 8
		}},
		{"row0-phase", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[0]["pts"] = 1
		}},
		{"row1-phase", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[1]["pts"] = 17
		}},
		{"row2-phase", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[2]["pts"] = 1040
		}},
		{"normalized-row2-phase", func(f *remainingNonKeySourceAudioFixture) {
			f.normalized = strings.Replace(f.normalized, "0, 1032, 1032,", "0, 1040, 1040,", 1)
		}},
		{"prime-pts", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[0]["pts"] = -1024
		}},
		{"prime-skip", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[0]["side_data_list"].([]map[string]any)[0]["skip_samples"] = 1024
		}},
		{"second-duration", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[1]["duration"] = 1024
		}},
		{"duplicate-prime-side-data", func(f *remainingNonKeySourceAudioFixture) {
			row := remainingNonKeySourceRows(*f, "packet")[0]
			row["side_data_list"] = append(row["side_data_list"].([]map[string]any), row["side_data_list"].([]map[string]any)[0])
		}},
		{"unknown-prime-side-data", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[0]["side_data_list"].([]map[string]any)[0]["side_data_type"] = "Unknown delay"
		}},
		{"missing-packet-dts", func(f *remainingNonKeySourceAudioFixture) {
			delete(remainingNonKeySourceRows(*f, "packet")[10], "dts")
		}},
		{"malformed-packet-hash", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[10]["data_hash"] = "SHA256:0"
		}},
		{"wrong-source-profile", func(f *remainingNonKeySourceAudioFixture) {
			f.native["streams"].([]map[string]any)[0]["profile"] = "HE-AAC"
		}},
		{"two-source-streams", func(f *remainingNonKeySourceAudioFixture) {
			f.native["streams"] = append(f.native["streams"].([]map[string]any), f.native["streams"].([]map[string]any)[0])
		}},
		{"normalized-wrong-size", func(f *remainingNonKeySourceAudioFixture) {
			f.normalized = strings.Replace(f.normalized, "0, 1032, 1032, 1024, 4096,", "0, 1032, 1032, 1024, 4092,", 1)
		}},
	}
	for _, value := range cases {
		t.Run(value.name, func(t *testing.T) {
			fixture := remainingNonKeySourceAudio(t, 48000, 16)
			value.damage(&fixture)
			if _, err := deriveCopiedHLSSourceAudio(t.Context(), fixture.encode(t), []byte(fixture.normalized), fixture.first, 12_500_000, fixture.edit); err == nil {
				t.Fatal("nonkey damaged partial source-bootstrap admitted")
			}
		})
	}
}
