package server

import "strings"

func ignoreNonPasswordSecretAutofill(page string) string {
	const ignored = `autocomplete="off"`
	return strings.ReplaceAll(page, ignored, ignored+` data-1p-ignore data-bwignore data-lpignore="true" data-form-type="other" spellcheck="false"`)
}
