package servertest

import "testing"

func TestAPIKeySecretIgnoresKeyShapedCSRFText(t *testing.T) {
	const csrf = `<meta name="kinosail-csrf" content="v8fKuTqTpG515Rcw-ks_GOdOoM4aNiOp-I-ep0ItaaCvNG8">`
	const issued = "ks_KL6XFUUARP5X6ZFEQTIWZODNH6"
	for name, body := range map[string]string{
		"player":    `<head>` + csrf + `</head><body><input id="api-key-secret" dir="ltr" value="` + issued + `" readonly></body>`,
		"subtitles": `<head>` + csrf + `</head><body><code class="grant-link">` + issued + `</code></body>`,
	} {
		if got := APIKeySecret(body); got != issued {
			t.Errorf("%s secret = %q, want %q", name, got, issued)
		}
	}
	for name, body := range map[string]string{
		"missing":   `<head>` + csrf + `</head>`,
		"truncated": `<code class="grant-link">ks_KL6XFUUARP5X6ZFEQTIWZODNH</code>`,
		"ambiguous": `<code class="grant-link">` + issued + `</code><code class="grant-link">ks_AAAAAAAAAAAAAAAAAAAAAAAAAA</code>`,
	} {
		if got := APIKeySecret(body); got != "" {
			t.Errorf("%s secret = %q, want none", name, got)
		}
	}
}
