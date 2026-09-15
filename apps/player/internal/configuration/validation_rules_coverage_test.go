package configuration

import "testing"

func TestValidationRuleRemainingBranches(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		validate func() error
		wantErr  bool
	}{
		"unknown kind":              {func() error { return validateKind(kind("unknown"), "") }, false},
		"valid server name":         {func() error { return validateServerName("Living Room") }, false},
		"empty server name":         {func() error { return validateServerName("") }, true},
		"valid subtitle language":   {func() error { return validateSubtitleLanguage("EN") }, false},
		"invalid subtitle language": {func() error { return validateSubtitleLanguage("not_a_language") }, true},
		"empty subtitle language":   {func() error { return validateSubtitleLanguage("") }, false},
		"positive duration":         {func() error { return validatePositiveDuration("1h") }, false},
		"zero duration":             {func() error { return validatePositiveDuration("0s") }, true},
		"malformed duration":        {func() error { return validatePositiveDuration("later") }, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := test.validate(); (err != nil) != test.wantErr {
				t.Fatalf("error = %v; want error %t", err, test.wantErr)
			}
		})
	}
}
