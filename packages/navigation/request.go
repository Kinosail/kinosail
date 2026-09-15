package navigation

import (
	"mime"
	"net/http"
	"strings"
)

const (
	maximumAcceptLength = 4096
	maximumMediaTypes   = 32
)

// IsBrowserRequest reports whether a bounded request expects an HTML navigation.
func IsBrowserRequest(request *http.Request, authenticationError bool) bool { //nolint:cyclop // The score of 15 remains below the repository ceiling of 22 for one HTTP negotiation predicate.
	if request == nil || authenticationError || request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodPost {
		return false
	}
	if request.Header.Get("Sec-Fetch-Dest") == "document" || request.Header.Get("Sec-Fetch-Mode") == "navigate" {
		return true
	}
	accept := request.Header.Get("Accept")
	if len(accept) > maximumAcceptLength || strings.Count(accept, ",") >= maximumMediaTypes {
		return false
	}
	for value := range strings.SplitSeq(accept, ",") {
		mediaType, parameters, err := mime.ParseMediaType(strings.TrimSpace(value))
		if err != nil || !strings.EqualFold(mediaType, "text/html") {
			continue
		}
		quality, specified := parameters["q"]
		if !specified || positiveHTTPQuality(quality) {
			return true
		}
	}
	return false
}

func positiveHTTPQuality(value string) bool { //nolint:cyclop // The score of 14 remains below the repository ceiling of 22 for one strict quality parser.
	if value == "0" {
		return false
	}
	if value == "1" {
		return true
	}
	if len(value) < 2 || len(value) > 5 || value[1] != '.' || value[0] != '0' && value[0] != '1' {
		return false
	}
	digits := value[2:]
	for _, digit := range digits {
		if digit < '0' || digit > '9' || value[0] == '1' && digit != '0' {
			return false
		}
	}
	return value[0] == '1' || strings.Trim(digits, "0") != ""
}
