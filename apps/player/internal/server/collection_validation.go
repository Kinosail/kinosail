package server

import (
	"encoding/hex"
	"net/url"
	"strings"
	"unicode/utf8"
)

func validCollectionPathName(name string) bool {
	return name != "" && len(name) <= 200 && utf8.ValidString(name) && !strings.Contains(name, "/")
}

func validCurationItemID(id string) bool {
	_, err := hex.DecodeString(id)
	return len(id) == 16 && err == nil
}

func validCurationItemForm(form url.Values) bool {
	if !validCurationItemFields(form) {
		return false
	}
	if values := form["included"]; len(values) != 1 || values[0] != "true" && values[0] != "false" {
		return false
	}
	query := form["q"]
	return len(query) <= 1 && (len(query) == 0 || len(query[0]) <= 200 && utf8.ValidString(query[0]))
}

func validCurationItemFields(form url.Values) bool {
	if csrf := form["_csrf"]; len(csrf) > 0 && (len(csrf) != 1 || len(csrf[0]) > 128) {
		return false
	}
	for key := range form {
		if key != "included" && key != "q" && key != "_csrf" {
			return false
		}
	}
	return true
}
