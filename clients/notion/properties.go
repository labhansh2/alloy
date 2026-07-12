package notion

import (
	"encoding/json"
	"strings"
)

// PageTitle returns the plain-text title of a page.
func PageTitle(page *Page) string {
	if page == nil {
		return ""
	}
	for _, prop := range page.Properties {
		if prop.Type != "title" {
			continue
		}
		var raw struct {
			Title []RichText `json:"title"`
		}
		if err := json.Unmarshal(prop.Raw, &raw); err != nil {
			return ""
		}
		return richTextPlain(raw.Title)
	}
	return ""
}

func richTextPlain(rt []RichText) string {
	var b strings.Builder
	for _, t := range rt {
		b.WriteString(t.PlainText)
	}
	return b.String()
}

// DataSourceTitle returns the plain-text title of a data source.
func DataSourceTitle(ds *DataSource) string {
	if ds == nil {
		return ""
	}
	return richTextPlain(ds.Title)
}

// MultiSelectOption is a selectable option on a multi_select property.
type MultiSelectOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// FindMultiSelectProperty returns the first multi_select property on a data
// source. PreferName, when set, is matched case-insensitively first.
func FindMultiSelectProperty(ds *DataSource, preferName string) (name string, options []MultiSelectOption, ok bool) {
	if ds == nil {
		return "", nil, false
	}
	preferName = strings.TrimSpace(preferName)

	type schema struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		MultiSelect *struct {
			Options []MultiSelectOption `json:"options"`
		} `json:"multi_select"`
	}

	var fallbackName string
	var fallbackOpts []MultiSelectOption

	for key, raw := range ds.Properties {
		var s schema
		if err := json.Unmarshal(raw, &s); err != nil || s.Type != "multi_select" || s.MultiSelect == nil {
			continue
		}
		propName := s.Name
		if propName == "" {
			propName = key
		}
		if preferName != "" && strings.EqualFold(propName, preferName) {
			return propName, s.MultiSelect.Options, true
		}
		if fallbackName == "" {
			fallbackName = propName
			fallbackOpts = s.MultiSelect.Options
		}
	}
	if fallbackName == "" {
		return "", nil, false
	}
	return fallbackName, fallbackOpts, true
}

// MultiSelectNames returns option names only.
func MultiSelectNames(options []MultiSelectOption) []string {
	names := make([]string, 0, len(options))
	for _, o := range options {
		if o.Name != "" {
			names = append(names, o.Name)
		}
	}
	return names
}

// MultiSelectPropertyUpdate builds an UpdatePage properties value for tags.
func MultiSelectPropertyUpdate(names []string) map[string]any {
	opts := make([]map[string]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		opts = append(opts, map[string]string{"name": n})
	}
	return map[string]any{
		"multi_select": opts,
	}
}
