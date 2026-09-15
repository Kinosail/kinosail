package supporter

import (
	"errors"
	"testing"
)

func TestNewActivationInputNormalizesAndValidatesFormValues(t *testing.T) {
	input, err := NewActivationInput("  VALID_SUPPORTER_KEY  ", "")
	if err != nil || input.Key != "VALID_SUPPORTER_KEY" || input.RecognitionName != nil {
		t.Fatalf("empty recognition input = %#v, %v", input, err)
	}
	name := "Quiet Supporter"
	input, err = NewActivationInput("VALID_SUPPORTER_KEY", name)
	if err != nil || input.RecognitionName == nil || *input.RecognitionName != name {
		t.Fatalf("named input = %#v, %v", input, err)
	}
	if _, err = NewActivationInput("short", ""); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid key error = %v", err)
	}
	if _, err = NewActivationInput("VALID_SUPPORTER_KEY", " padded "); !errors.Is(err, ErrInvalidRecognitionName) {
		t.Fatalf("invalid recognition error = %v", err)
	}
}

func TestJSONObjectRejectsMalformedValuesAndTrailingData(t *testing.T) {
	for _, input := range []string{`{"field":}`, `{"field":true`, `{"field":true} {}`} {
		if _, err := jsonObject([]byte(input)); err == nil {
			t.Fatalf("jsonObject(%q) succeeded", input)
		}
	}
}

func TestUnicodeEscapeValidationCoversEveryEscapeShape(t *testing.T) {
	tests := []struct {
		name   string
		record string
		valid  bool
	}{
		{"outside string", `\x`, true},
		{"ordinary escape", `"\n"`, true},
		{"truncated unicode", `"\u12"`, false},
		{"unexpected low surrogate", `"\udc00"`, false},
		{"basic plane", `"\u0041"`, true},
		{"paired surrogate", `"\ud800\udc00"`, true},
		{"missing low surrogate", `"\ud800A"`, false},
		{"invalid low surrogate", `"\ud800\u0041"`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validJSONUnicode([]byte(test.record)); got != test.valid {
				t.Fatalf("validJSONUnicode(%q) = %t", test.record, got)
			}
		})
	}
}

func TestDisplayAndStrictTimeFallbacks(t *testing.T) {
	if Name("") != "Free" || Name("free") != "Free" {
		t.Fatal("free display names changed")
	}
	if _, ok := strictTime("not-a-time"); ok {
		t.Fatal("strictTime accepted malformed input")
	}
}
