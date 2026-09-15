package updatecontrol

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func releasePolicy(app string, stateSchema, configurationSchema int) Policy {
	return Policy{
		tagPrefix:           app,
		signatureIdentity:   fmt.Sprintf(`^https://github\.com/MikeO7/kinosail/\.github/workflows/%s-release\.yml@refs/tags/%s-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`, app, app),
		stateSchema:         stateSchema,
		configurationSchema: configurationSchema,
	}
}

func validatePolicy(policy Policy) error {
	if !slices.Contains([]string{"player", "subtitles"}, policy.tagPrefix) || policy.signatureIdentity != releasePolicy(policy.tagPrefix, policy.stateSchema, policy.configurationSchema).signatureIdentity || !validSchemaRange(1, policy.stateSchema) || !validSchemaRange(1, policy.configurationSchema) {
		return errors.New("invalid update release policy")
	}
	return nil
}

func ValidReleaseVersion(value string) bool {
	if !strings.HasPrefix(value, "v") || len(value) > 64 {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return false
		}
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return false
		}
	}
	return true
}

func releaseManifestURLs(version string, policy Policy) (string, string) {
	if !ValidReleaseVersion(version) {
		return "", ""
	}
	base := fmt.Sprintf("https://github.com/MikeO7/kinosail/releases/download/%s-%s/kinosail-release.json", policy.tagPrefix, version)
	return base, base + ".sigstore.json"
}

func validTransition(from, to string) bool {
	allowed := map[string]string{
		"":           "checking available installing failed rolled-back current",
		"checking":   "available current failed",
		"available":  "installing current failed",
		"installing": "current failed rolled-back",
		"failed":     "checking installing rolled-back",
	}
	return strings.Contains(" "+allowed[from]+" ", " "+to+" ")
}
