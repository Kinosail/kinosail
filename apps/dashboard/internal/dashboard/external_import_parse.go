package dashboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"

	"go.yaml.in/yaml/v3"
)

func decodeConfig(content string) (any, error) {
	var value any
	if json.Unmarshal([]byte(content), &value) == nil {
		return value, nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(content)))
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("import must contain exactly one YAML document")
	}
	return value, nil
}

func dashyApps(value any) (string, []ImportedApp) {
	root := mapValue(value, "")
	title := stringValue(mapValue(root, "pageInfo"), "title")
	var apps []ImportedApp
	for _, section := range listValue(root, "sections") {
		sectionMap := mapValue(section, "")
		category := stringValue(sectionMap, "name")
		for _, item := range listValue(sectionMap, "items") {
			itemMap := mapValue(item, "")
			apps = append(apps, importedApp(stringValue(itemMap, "title"), firstString(itemMap, "localUrl", "url"), stringValue(itemMap, "description"), category))
		}
	}
	return title, compactApps(apps)
}

func homepageApps(value any) []ImportedApp {
	root := mapValue(value, "")
	var apps []ImportedApp
	for category, group := range root {
		for _, entry := range listItems(group) {
			for name, details := range entry {
				detailMap := mapValue(details, "")
				apps = append(apps, importedApp(name, firstString(detailMap, "href", "url"), stringValue(detailMap, "description"), category))
			}
		}
	}
	return compactApps(apps)
}

func genericApps(value any) []ImportedApp {
	root := mapValue(value, "")
	var apps []ImportedApp
	for _, key := range []string{"apps", "items", "services"} {
		for _, item := range listValue(root, key) {
			itemMap := mapValue(item, "")
			name := firstString(itemMap, "name", "title", "label")
			apps = append(apps, importedApp(name, firstString(itemMap, "url", "href", "link"), stringValue(itemMap, "description"), firstString(itemMap, "category", "group", "section", "categoryName", "name")))
		}
	}
	return compactApps(apps)
}

func importedApp(name, address, description, category string) ImportedApp {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	if name == "" {
		if parsed, err := url.Parse(address); err == nil {
			name = parsed.Hostname()
		}
	}
	identity := matchCatalog(name, address)
	app := ImportedApp{Name: name, URL: address, Description: strings.TrimSpace(description), Category: strings.TrimSpace(category), Icon: "app", Accent: "slate", CheckEnabled: true}
	if identity != nil {
		app.Icon, app.Accent = identity.Icon, identity.Accent
		if app.Description == "" {
			app.Description = identity.Description
		}
		if app.Category == "" {
			app.Category = identity.Category
		}
	}
	return app
}

func matchCatalog(name, address string) *CatalogEntry {
	needle := strings.ToLower(name + " " + address)
	for index := range catalog {
		entry := &catalog[index]
		if strings.Contains(needle, entry.ID) || strings.Contains(needle, strings.ToLower(entry.Name)) {
			return entry
		}
	}
	return nil
}

func compactApps(apps []ImportedApp) []ImportedApp {
	result := make([]ImportedApp, 0, len(apps))
	seen := map[string]bool{}
	for _, app := range apps {
		if app.Name == "" || app.URL == "" || strings.ContainsAny(app.Name, "\r\n") || strings.ContainsAny(app.URL, "\r\n") || seen[strings.ToLower(app.URL)] {
			continue
		}
		seen[strings.ToLower(app.URL)] = true
		result = append(result, app)
	}
	return result
}

func mapValue(value any, key string) map[string]any {
	if key == "" {
		if result, ok := value.(map[string]any); ok {
			return result
		}
		return nil
	}
	root := mapValue(value, "")
	return mapValue(root[key], "")
}

func listValue(value any, key string) []any {
	root := mapValue(value, "")
	items, _ := root[key].([]any)
	return items
}

func listItems(value any) []map[string]any {
	items, _ := value.([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mapped := mapValue(item, ""); mapped != nil {
			result = append(result, mapped)
		}
	}
	return result
}

func stringValue(value any, key string) string {
	root := mapValue(value, "")
	result, _ := root[key].(string)
	return strings.TrimSpace(result)
}

func firstString(value any, keys ...string) string {
	for _, key := range keys {
		if result := stringValue(value, key); result != "" {
			return result
		}
	}
	return ""
}
