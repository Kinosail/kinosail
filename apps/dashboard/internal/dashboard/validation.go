package dashboard

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var tokenPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

var accents = []string{"lime", "ocean", "amber", "coral", "sky", "violet", "slate"}

func normalizeCreate(input CreateInput) (App, error) { //nolint:cyclop // All related fields are validated before construction.
	var err error
	input, err = withCatalogDefaults(input)
	if err != nil {
		return App{}, err
	}
	if strings.TrimSpace(input.Icon) == "" {
		input.Icon = "app"
	}
	if strings.TrimSpace(input.Accent) == "" {
		input.Accent = "slate"
	}
	name, err := boundedText("name", input.Name, 1, 60)
	if err != nil {
		return App{}, err
	}
	address, err := normalizeURL("url", input.URL, true)
	if err != nil {
		return App{}, err
	}
	health := input.HealthURL
	if strings.TrimSpace(health) == "" {
		health = address
	}
	health, err = normalizeURL("healthUrl", health, true)
	if err != nil {
		return App{}, err
	}
	return normalizedApp(input, name, address, health)
}

func withCatalogDefaults(input CreateInput) (CreateInput, error) {
	if input.CatalogID == "" {
		return input, nil
	}
	entry, found := catalogByID(strings.TrimSpace(input.CatalogID))
	if !found {
		return CreateInput{}, errors.New("catalogId is not supported")
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = entry.Name
	}
	if strings.TrimSpace(input.Description) == "" {
		input.Description = entry.Description
	}
	if strings.TrimSpace(input.Category) == "" {
		input.Category = entry.Category
	}
	if strings.TrimSpace(input.Icon) == "" {
		input.Icon = entry.Icon
	}
	if strings.TrimSpace(input.Accent) == "" {
		input.Accent = entry.Accent
	}
	return input, nil
}

func normalizedApp(input CreateInput, name, address, health string) (App, error) {
	description, err := boundedText("description", input.Description, 0, 120)
	if err != nil {
		return App{}, err
	}
	category, err := boundedText("category", input.Category, 0, 40)
	if err != nil {
		return App{}, err
	}
	icon, accent, err := normalizeStyle(input.Icon, input.Accent)
	if err != nil {
		return App{}, err
	}
	return App{Name: name, URL: address, HealthURL: health, CheckEnabled: input.CheckEnabled, Description: description, Category: category, Icon: icon, Accent: accent, Favorite: input.Favorite}, nil
}

func normalizeStyle(iconValue, accentValue string) (string, string, error) {
	icon := strings.ToLower(strings.TrimSpace(iconValue))
	accent := strings.ToLower(strings.TrimSpace(accentValue))
	if len(icon) > 32 || !tokenPattern.MatchString(icon) {
		return "", "", errors.New("icon must be a supported token")
	}
	if !slices.Contains(accents, accent) {
		return "", "", errors.New("accent is not supported")
	}
	return icon, accent, nil
}

func normalizeURL(field, value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && !required {
		return "", nil
	}
	if value == "" || len(value) > 2048 || strings.ContainsAny(value, "\r\n\t#") {
		return "", errors.New(field + " must be a valid HTTP or HTTPS address")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || invalidParsedURL(parsed) {
		return "", errors.New(field + " must be a valid HTTP or HTTPS address without credentials, a query, or a fragment")
	}
	if !validURLPath(parsed) {
		return "", errors.New(field + " contains an invalid path")
	}
	port, err := validPort(parsed.Port())
	if err != nil {
		return "", errors.New(field + " contains an invalid port")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = normalizedURLHost(parsed.Scheme, parsed.Hostname(), port)
	return parsed.String(), nil
}

func validURLPath(parsed *url.URL) bool {
	decodedPath, err := url.PathUnescape(parsed.EscapedPath())
	return err == nil && !strings.ContainsRune(decodedPath, '#') && strings.IndexFunc(decodedPath, unicode.IsControl) < 0
}

func normalizedURLHost(scheme, hostname, port string) string {
	hostname = strings.ToLower(hostname)
	var host string
	if port == "" || scheme == "http" && port == "80" || scheme == "https" && port == "443" {
		host = hostname
		if strings.Contains(hostname, ":") {
			host = "[" + hostname + "]"
		}
	} else {
		host = net.JoinHostPort(hostname, port)
	}
	return host
}

func invalidParsedURL(parsed *url.URL) bool {
	return parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || parsed.ForceQuery
}

func validPort(port string) (string, error) {
	if port == "" {
		return "", nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", errors.New("invalid port")
	}
	return port, nil
}

func boundedText(field, value string, minimum, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || length < minimum || length > maximum {
		return "", errors.New(field + " has an invalid length")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", errors.New(field + " contains control characters")
		}
	}
	return value, nil
}
