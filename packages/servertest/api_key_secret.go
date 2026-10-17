package servertest

import "regexp"

var createdAPIKeySecret = regexp.MustCompile(`(?:id="api-key-secret"[^>]*\svalue="|<code class="grant-link">)(ks_[A-Z2-7]{26})["<]`)

func APIKeySecret(body string) string {
	matches := createdAPIKeySecret.FindAllStringSubmatch(body, 2)
	if len(matches) != 1 {
		return ""
	}
	return matches[0][1]
}
