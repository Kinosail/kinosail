package httpguard

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

func LogMethod(method string) string {
	switch method {
	case http.MethodGet:
		return http.MethodGet
	case http.MethodHead:
		return http.MethodHead
	case http.MethodPost:
		return http.MethodPost
	case http.MethodPut:
		return http.MethodPut
	case http.MethodPatch:
		return http.MethodPatch
	case http.MethodDelete:
		return http.MethodDelete
	case http.MethodOptions:
		return http.MethodOptions
	default:
		return "OTHER"
	}
}

func LogText(value string, limit int) string {
	if limit < 1 {
		return ""
	}
	if len(value) > 4*limit {
		value = value[:4*limit]
	}
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", "")
	value = strings.Map(func(character rune) rune {
		if character == utf8.RuneError || unicode.IsControl(character) || unicode.Is(unicode.Zl, character) || unicode.Is(unicode.Zp, character) {
			return -1
		}
		return character
	}, value)
	if len(value) > limit {
		value = strings.ToValidUTF8(value[:limit], "")
	}
	return value
}
