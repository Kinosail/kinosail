package credentials_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/credentials"
	"golang.org/x/crypto/argon2"
)

const strongPassword = "windward-owner-2026"

func TestPolicyRejectsInvalidPasswords(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"missing":    "",
		"short":      "short",
		"oversized":  strings.Repeat("a", 1025),
		"common":     "passwordpassword",
		"normalized": "  QWERTYQWERTY  ",
	}
	for name, password := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := credentials.Validate(password); err == nil {
				t.Fatal("invalid password was accepted")
			}
			if encoded, err := credentials.Hash(password); err == nil || encoded != "" {
				t.Fatalf("invalid password hash = %q, %v", encoded, err)
			}
		})
	}
	if err := credentials.Validate(strongPassword); err != nil {
		t.Fatalf("strong password rejected: %v", err)
	}
}

func TestCurrentCredentialRoundTrip(t *testing.T) {
	t.Parallel()
	encoded, err := credentials.Hash(strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("credential format = %q", encoded)
	}
	if !credentials.Verify(encoded, strongPassword) || credentials.Verify(encoded, "wrong-password-value") {
		t.Fatal("current credential verification changed")
	}
}

func TestStrictPHCParsingRejectsMalformedAndOversizedCredentials(t *testing.T) {
	t.Parallel()
	encoded, err := credentials.Hash(strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(encoded, "$")
	tests := map[string]string{
		"missing":              "",
		"wrong scheme":         strings.Replace(encoded, "argon2id", "argon2i", 1),
		"wrong version":        strings.Replace(encoded, "v=19", "v=16", 1),
		"wrong memory":         strings.Replace(encoded, "m=19456", "m=19457", 1),
		"wrong iterations":     strings.Replace(encoded, "t=2", "t=3", 1),
		"wrong parallelism":    strings.Replace(encoded, "p=1", "p=2", 1),
		"reordered parameters": strings.Replace(encoded, "m=19456,t=2,p=1", "t=2,m=19456,p=1", 1),
		"trailing parameter":   strings.Replace(encoded, "p=1", "p=1,extra=1", 1),
		"invalid salt":         strings.Join([]string{"", "argon2id", "v=19", "m=19456,t=2,p=1", "!", parts[5]}, "$"),
		"noncanonical salt":    strings.Join([]string{"", "argon2id", "v=19", "m=19456,t=2,p=1", strings.Repeat("A", 21) + "B", parts[5]}, "$"),
		"short salt":           strings.Join([]string{"", "argon2id", "v=19", "m=19456,t=2,p=1", base64.RawStdEncoding.EncodeToString([]byte("short")), parts[5]}, "$"),
		"invalid hash":         strings.Join([]string{"", "argon2id", "v=19", "m=19456,t=2,p=1", parts[4], "!"}, "$"),
		"short hash":           strings.Join([]string{"", "argon2id", "v=19", "m=19456,t=2,p=1", parts[4], base64.RawStdEncoding.EncodeToString([]byte("short"))}, "$"),
		"extra field":          encoded + "$extra",
		"oversized":            strings.Repeat("x", 4097),
	}
	for name, malformed := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if credentials.Verify(malformed, strongPassword) {
				t.Fatal("malformed credential was accepted")
			}
		})
	}
	if credentials.Verify(encoded, strings.Repeat("x", 1025)) {
		t.Fatal("oversized password was accepted")
	}
}

func TestLegacyCredentialsRemainVerifiable(t *testing.T) {
	t.Parallel()
	salt := []byte("abcdefghijklmnop")
	encoded := base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(argon2.IDKey([]byte("legacy-password"), salt, 2, 19*1024, 1, 32))
	if !credentials.Verify(encoded, "legacy-password") || credentials.Verify(encoded, "wrong-password") {
		t.Fatal("legacy credential verification changed")
	}
	for _, malformed := range []string{"missing-parts", "!" + encoded, encoded + ".extra"} {
		if credentials.Verify(malformed, "legacy-password") {
			t.Fatal("malformed legacy credential was accepted")
		}
	}
}

func TestDummyVerificationAcceptsBoundedAndOversizedInput(t *testing.T) {
	t.Parallel()
	credentials.DummyVerify("unknown-password")
	credentials.DummyVerify(strings.Repeat("x", 1025))
}
