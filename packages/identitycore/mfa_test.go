package identitycore

import (
	"bytes"
	"encoding/base32"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type countingReader struct {
	reads int
	err   error
}

func (reader *countingReader) Read(value []byte) (int, error) {
	reader.reads++
	if reader.err != nil {
		return 0, reader.err
	}
	for index := range value {
		value[index] = 1
	}
	return len(value), nil
}

func TestMFASetupValidatesBeforeRandomness(t *testing.T) {
	t.Parallel()
	reader := &countingReader{}
	engine := &MFA{pending: make(map[string]Enrollment), now: time.Now, random: reader}
	for _, input := range []struct{ identity, name string }{
		{"", "Owner"},
		{strings.Repeat("i", maxIdentityLength+1), "Owner"},
		{"owner", ""},
		{"owner", strings.Repeat("n", maxNameLength+1)},
	} {
		if enrollment, err := engine.Setup(input.identity, input.name); !errors.Is(err, ErrInvalidInput) || enrollment.Secret != "" || enrollment.URI != "" || enrollment.RecoveryCodes != nil || !enrollment.Expires.IsZero() {
			t.Fatalf("Setup(%q, %q) = %#v, %v", input.identity, input.name, enrollment, err)
		}
	}
	if reader.reads != 0 || engine.PendingCount() != 0 {
		t.Fatalf("invalid setup performed side effects: reads=%d pending=%d", reader.reads, engine.PendingCount())
	}
}

func TestMFASetupAcceptsExactIdentityBounds(t *testing.T) {
	t.Parallel()
	identity, name := strings.Repeat("i", maxIdentityLength), strings.Repeat("n", maxNameLength)
	engine := &MFA{pending: make(map[string]Enrollment), now: time.Now, random: bytes.NewReader(make([]byte, 100))}
	if enrollment, err := engine.Setup(identity, name); err != nil || enrollment.Secret == "" || engine.PendingCount() != 1 {
		t.Fatalf("bounded setup = %#v, %v", enrollment, err)
	}
	for index, invalid := range []*MFA{
		{pending: make(map[string]Enrollment), random: bytes.NewReader(make([]byte, 100))},
		{pending: make(map[string]Enrollment), now: time.Now},
	} {
		if _, err := invalid.Setup("owner", "Owner"); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid MFA dependency %d error = %v", index, err)
		}
	}
}

func TestMFASetupBuildsIsolatedEnrollment(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	engine := &MFA{pending: make(map[string]Enrollment), now: func() time.Time { return now }, random: bytes.NewReader(make([]byte, 100))}
	enrollment, err := engine.Setup("owner", "Owner Profile")
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.Secret != strings.Repeat("A", 32) || !strings.Contains(enrollment.URI, "Kinosail:Owner+Profile") || len(enrollment.RecoveryCodes) != recoveryCodeCount || !enrollment.Expires.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("unexpected enrollment: %#v", enrollment)
	}
	enrollment.RecoveryCodes[0] = "changed"
	if engine.PendingCount() != 1 {
		t.Fatal("setup did not store the enrollment")
	}
	stored := engine.pending["owner"]
	if stored.RecoveryCodes[0] == "changed" {
		t.Fatal("returned recovery codes alias stored state")
	}
}

func TestMFASetupRandomFailureDoesNotStoreEnrollment(t *testing.T) {
	t.Parallel()
	engine := &MFA{pending: make(map[string]Enrollment), now: time.Now, random: &countingReader{err: io.ErrUnexpectedEOF}}
	if _, err := engine.Setup("owner", "Owner"); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("setup error = %v", err)
	}
	if engine.PendingCount() != 0 {
		t.Fatal("failed setup stored an enrollment")
	}
}

func TestMFAConstructorAndRecoveryRandomFailure(t *testing.T) {
	t.Parallel()
	engine := NewMFA()
	if engine == nil || engine.pending == nil || engine.now == nil || engine.random == nil {
		t.Fatalf("new MFA engine = %#v", engine)
	}
	engine = &MFA{pending: make(map[string]Enrollment), now: time.Now, random: bytes.NewReader(make([]byte, 20))}
	if _, err := engine.Setup("owner", "Owner"); !errors.Is(err, io.EOF) || engine.PendingCount() != 0 {
		t.Fatalf("recovery randomness failure = %v, pending=%d", err, engine.PendingCount())
	}
}

func TestMFAConfirmValidatesAndConsumesOnlyValidEnrollment(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	secretBytes := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	engine := &MFA{
		pending: map[string]Enrollment{"owner": {Secret: secret, RecoveryCodes: []string{"AAAA-BBBB-CCCC-DDDD"}, Expires: now.Add(time.Minute)}},
		now:     func() time.Time { return now },
		random:  bytes.NewReader(nil),
	}
	for _, input := range []struct{ identity, code string }{
		{"", "000000"},
		{strings.Repeat("i", maxIdentityLength+1), "000000"},
		{"owner", strings.Repeat("1", 33)},
		{"owner", "not-a-code"},
	} {
		if _, err := engine.Confirm(input.identity, input.code); !errors.Is(err, ErrInvalidEnrollment) {
			t.Fatalf("Confirm(%q, %q) error = %v", input.identity, input.code, err)
		}
		if engine.PendingCount() != 1 {
			t.Fatal("invalid confirmation consumed the enrollment")
		}
	}
	code := totp(secretBytes, now.Unix()/30)
	enrollment, err := engine.Confirm("owner", code)
	if err != nil || enrollment.Secret != secret {
		t.Fatalf("valid confirmation = %#v, %v", enrollment, err)
	}
	if engine.PendingCount() != 0 {
		t.Fatal("valid confirmation retained the enrollment")
	}
	enrollment.RecoveryCodes[0] = "changed"
}

func TestMFAConfirmIsolatesMissingExpiredAndInvalidCodes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	key := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	code := totp(key, now.Unix()/30)
	for name, enrollment := range map[string]map[string]Enrollment{
		"missing":      {},
		"expired":      {"owner": {Secret: secret, Expires: now.Add(-time.Nanosecond)}},
		"invalid code": {"owner": {Secret: secret, Expires: now.Add(time.Minute)}},
	} {
		engine := &MFA{pending: enrollment, now: func() time.Time { return now }, random: bytes.NewReader(nil)}
		candidate := code
		if name == "invalid code" {
			candidate = "000000"
		}
		if _, err := engine.Confirm("owner", candidate); !errors.Is(err, ErrInvalidEnrollment) {
			t.Fatalf("%s confirmation error = %v", name, err)
		}
	}
	identity := strings.Repeat("i", maxIdentityLength)
	engine := &MFA{pending: map[string]Enrollment{identity: {Secret: secret, Expires: now.Add(time.Minute)}}, now: func() time.Time { return now }}
	paddedCode := code + strings.Repeat(" ", 32-len(code))
	if _, err := engine.Confirm(identity, paddedCode); err != nil {
		t.Fatalf("bounded confirmation error = %v", err)
	}
}

func TestMFAExpiryDiscardAndNilEngine(t *testing.T) {
	t.Parallel()
	now := time.Now()
	engine := &MFA{pending: map[string]Enrollment{"owner": {Secret: "A", Expires: now.Add(-time.Second)}}, now: func() time.Time { return now }, random: bytes.NewReader(nil)}
	if _, err := engine.Confirm("owner", "000000"); !errors.Is(err, ErrInvalidEnrollment) || engine.PendingCount() != 1 {
		t.Fatalf("expired confirmation = %v, pending=%d", err, engine.PendingCount())
	}
	engine.Discard(strings.Repeat("i", maxIdentityLength+1))
	if engine.PendingCount() != 1 {
		t.Fatal("invalid discard changed state")
	}
	boundedIdentity := strings.Repeat("i", maxIdentityLength)
	engine.pending[boundedIdentity] = Enrollment{}
	engine.Discard(boundedIdentity)
	if _, found := engine.pending[boundedIdentity]; found {
		t.Fatal("bounded identity was not discarded")
	}
	engine.Discard("owner")
	if engine.PendingCount() != 0 {
		t.Fatal("discard retained enrollment")
	}
	var missing *MFA
	missing.Discard("owner")
	if missing.PendingCount() != 0 {
		t.Fatal("nil engine reported pending state")
	}
	if _, err := missing.Setup("owner", "Owner"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil setup error = %v", err)
	}
	if _, err := missing.Confirm("owner", "000000"); !errors.Is(err, ErrInvalidEnrollment) {
		t.Fatalf("nil confirmation error = %v", err)
	}
}

func TestValidTOTPAndRecoveryNormalization(t *testing.T) { //nolint:cyclop,gocognit // Exact cryptographic and input boundaries share one fixed RFC fixture.
	t.Parallel()
	totpKey := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" //nolint:gosec // G101: this is the public RFC 6238 test key.
	if !ValidTOTP(totpKey, "287082", time.Unix(59, 0)) || !ValidTOTP(totpKey, "287082 ", time.Unix(89, 0)) {
		t.Fatal("RFC 6238 code or adjacent time step was rejected")
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(totpKey)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Unix(59, 0)
	if !ValidTOTP(totpKey, totp(key, when.Unix()/30+1), when) {
		t.Fatal("next adjacent TOTP step was rejected")
	}
	maxKey := bytes.Repeat([]byte{1}, 80)
	maxSecret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(maxKey)
	if len(maxSecret) != maxSecretLength || !ValidTOTP(maxSecret, totp(maxKey, when.Unix()/30), when) {
		t.Fatal("maximum-length TOTP secret was rejected")
	}
	for _, invalid := range []struct{ secret, code string }{
		{"", "287082"},
		{strings.Repeat("A", maxSecretLength+1), "287082"},
		{"!", "287082"},
		{totpKey, ""},
		{totpKey, "12345"},
		{totpKey, "1234567"},
		{totpKey, "12a456"},
		{totpKey, "287082"},
	} {
		when := time.Unix(59, 0)
		if invalid.secret == totpKey && invalid.code == "287082" {
			when = time.Unix(-31, 0)
		}
		if ValidTOTP(invalid.secret, invalid.code, when) {
			t.Fatalf("invalid TOTP accepted: secret=%q code=%q", invalid.secret, invalid.code)
		}
	}
	if got := NormalizeRecovery(" aa-bb cc-dd "); got != "AABBCCDD" {
		t.Fatalf("normalized recovery code = %q", got)
	}
	if got := NormalizeRecovery(strings.Repeat("a", 129)); got != "" {
		t.Fatalf("oversized recovery code = %q", got)
	}
	if got := NormalizeRecovery(strings.Repeat("a", 128)); len(got) != 128 {
		t.Fatalf("bounded recovery code length = %d", len(got))
	}
	if totp([]byte("key"), 0) == "" || totp([]byte("key"), -1) != "" {
		t.Fatal("TOTP counter boundary changed")
	}
	for _, invalid := range []string{"/12345", ":12345"} {
		if decimal(invalid) {
			t.Fatalf("non-decimal code %q accepted", invalid)
		}
	}
}
