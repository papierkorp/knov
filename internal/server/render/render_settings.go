package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
)

// RenderSettingsSection renders the inner content of a settings section (h2 + grouped items).
// It does NOT include the outer <section> wrapper — the caller's HTMX hx-swap="innerHTML"
// fills that element directly.
// afterUngrouped is optional HTML injected after ungrouped (GroupNone) items and before the first named group.
func RenderSettingsSection(section configmanager.SettingSection, t func(string, ...any) string, afterUngrouped ...string) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<h2>%s</h2>`, t(section.Label)))

	if desc := section.Description; desc != "" {
		html.WriteString(fmt.Sprintf(`<p class="section-description">%s</p>`, t(desc)))
	}

	items := configmanager.SettingsBySection(section)

	type groupEntry struct {
		group configmanager.SettingGroup
		items []configmanager.RenderableSetting
	}
	var groupOrder []configmanager.SettingGroup
	groupMap := make(map[configmanager.SettingGroup]*groupEntry)

	for _, s := range items {
		g := s.GetMeta().Group
		if _, exists := groupMap[g]; !exists {
			groupOrder = append(groupOrder, g)
			groupMap[g] = &groupEntry{group: g}
		}
		groupMap[g].items = append(groupMap[g].items, s)
	}

	injected := false
	for _, g := range groupOrder {
		entry := groupMap[g]
		if g == configmanager.GroupNone {
			for _, s := range entry.items {
				html.WriteString(renderSettingItem(s, t))
			}
			for _, extra := range afterUngrouped {
				html.WriteString(extra)
			}
			injected = true
		} else {
			if !injected {
				for _, extra := range afterUngrouped {
					html.WriteString(extra)
				}
				injected = true
			}
			html.WriteString(`<div class="setting-group">`)
			html.WriteString(fmt.Sprintf(`<h3>%s</h3>`, t(g.Label)))
			if desc := g.Description; desc != "" {
				html.WriteString(fmt.Sprintf(`<p class="section-description">%s</p>`, t(desc)))
			}
			for _, s := range entry.items {
				html.WriteString(renderSettingItem(s, t))
			}
			html.WriteString(`</div>`)
		}
	}
	if !injected {
		for _, extra := range afterUngrouped {
			html.WriteString(extra)
		}
	}

	return html.String()
}

// RenderFaviconItem renders the favicon upload setting item for the General settings section.
func RenderFaviconItem(t func(string, ...any) string) string {
	var html strings.Builder

	html.WriteString(`<div class="setting-item">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, t("Custom Favicon")))
	html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, t("upload a custom favicon (.ico, .png or .svg, max 2 MB)")))
	if ext := configmanager.GetCustomFaviconExt(); ext != "" {
		html.WriteString(fmt.Sprintf(
			`<div class="favicon-preview" style="margin-bottom: 1rem; display: flex; align-items: center; gap: 1rem;">`+
				`<img src="/favicon.ico" alt="current favicon" style="width: 32px; height: 32px; object-fit: contain;" />`+
				`<span>%s: %s</span>`+
				`<button class="btn-danger btn-sm"`+
				` hx-delete="/api/config/favicon"`+
				` hx-target="#favicon-status"`+
				` hx-confirm="%s">%s</button>`+
				`</div>`,
			t("current custom favicon"), ext,
			t("remove custom favicon and revert to default?"), t("remove"),
		))
	}
	html.WriteString(fmt.Sprintf(
		`<form hx-post="/api/config/favicon" hx-encoding="multipart/form-data" hx-target="#favicon-status">`+
			`<label for="favicon-file">%s`+
			`<input type="file" name="file" id="favicon-file" accept=".ico,.png,.svg" required class="form-input" />`+
			`</label>`+
			`<button type="submit" class="btn-primary" style="margin-top: 8px;">%s</button>`+
			`</form>`+
			`<div id="favicon-status"></div>`,
		t("choose file"), t("upload favicon"),
	))
	html.WriteString(`</div>`)

	return html.String()
}

func renderSettingItem(s configmanager.RenderableSetting, t func(string, ...any) string) string {
	var html strings.Builder
	html.WriteString(`<div class="setting-item">`)

	meta := s.GetMeta()
	postURL := "/api/settings/" + s.Key()

	trigger := meta.Trigger
	if trigger == "" {
		trigger = "change"
	}
	swap := "none"

	targetAttr := ""
	if meta.Target != "" {
		targetAttr = fmt.Sprintf(` hx-target="%s"`, meta.Target)
	}

	switch s.Type() {
	case "boolean":
		checked := ""
		if v, ok := s.GetValue().(bool); ok && v {
			checked = " checked"
		}
		html.WriteString(fmt.Sprintf(
			`<form hx-post="%s" hx-trigger="%s" hx-swap="%s"%s>`+
				`<label class="checkbox-label">`+
				`<input type="checkbox" name="%s" value="true" class="form-checkbox"%s />`+
				`<span class="checkmark"></span>`+
				`%s`+
				`<small>%s</small>`+
				`</label>`+
				`</form>`,
			postURL, trigger, swap, targetAttr,
			s.Key(), checked,
			t(meta.Label),
			t(meta.Desc),
		))

	case "select":
		currentVal := ""
		if v, ok := s.GetValue().(string); ok {
			currentVal = v
		}
		html.WriteString(fmt.Sprintf(`<form hx-post="%s" hx-trigger="%s" hx-swap="%s"%s>`, postURL, trigger, swap, targetAttr))
		html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, s.Key(), t(meta.Label)))
		html.WriteString(fmt.Sprintf(`<select name="%s" id="%s" class="form-select">`, s.Key(), s.Key()))
		for _, opt := range meta.Options {
			label := opt.Label
			if label == "" {
				label = opt.Value
			}
			selected := ""
			if opt.Value == currentVal {
				selected = " selected"
			}
			html.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, opt.Value, selected, t(label)))
		}
		html.WriteString(`</select>`)
		if meta.Desc != "" {
			html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, t(meta.Desc)))
		}
		html.WriteString(`</form>`)

	case "dynamic-select":
		html.WriteString(fmt.Sprintf(`<form hx-post="%s" hx-trigger="%s" hx-swap="%s"%s>`, postURL, trigger, swap, targetAttr))
		html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, s.Key(), t(meta.Label)))
		html.WriteString(fmt.Sprintf(
			`<select name="%s" id="%s" class="form-select" hx-get="%s" hx-trigger="load" hx-swap="innerHTML">`+
				`<option>%s</option>`+
				`</select>`,
			s.Key(), s.Key(), meta.DynURL, t("Loading..."),
		))
		if meta.Desc != "" {
			html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, t(meta.Desc)))
		}
		html.WriteString(`</form>`)

	case "text":
		currentVal := ""
		if v, ok := s.GetValue().(string); ok {
			currentVal = v
		}
		html.WriteString(fmt.Sprintf(`<form hx-post="%s" hx-trigger="%s" hx-swap="%s"%s>`, postURL, trigger, swap, targetAttr))
		html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, s.Key(), t(meta.Label)))
		html.WriteString(fmt.Sprintf(`<input type="text" name="%s" id="%s" value="%s" class="form-input" />`, s.Key(), s.Key(), currentVal))
		if meta.Target != "" {
			html.WriteString(fmt.Sprintf(`<div id="%s"></div>`, strings.TrimPrefix(meta.Target, "#")))
		}
		if meta.Desc != "" {
			html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, t(meta.Desc)))
		}
		html.WriteString(`</form>`)

	case "number":
		currentVal := 0
		switch v := s.GetValue().(type) {
		case int:
			currentVal = v
		case int64:
			currentVal = int(v)
		case float64:
			currentVal = int(v)
		}
		minAttr := ""
		if meta.Min != nil {
			minAttr = fmt.Sprintf(` min="%d"`, *meta.Min)
		}
		maxAttr := ""
		if meta.Max != nil {
			maxAttr = fmt.Sprintf(` max="%d"`, *meta.Max)
		}
		html.WriteString(fmt.Sprintf(`<form hx-post="%s" hx-trigger="%s" hx-swap="%s"%s>`, postURL, trigger, swap, targetAttr))
		html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, s.Key(), t(meta.Label)))
		html.WriteString(fmt.Sprintf(`<input type="number" name="%s" id="%s" value="%d" class="form-input"%s%s />`, s.Key(), s.Key(), currentVal, minAttr, maxAttr))
		if meta.Desc != "" {
			html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, t(meta.Desc)))
		}
		html.WriteString(`</form>`)

	case "textarea":
		currentVal := ""
		switch v := s.GetValue().(type) {
		case string:
			currentVal = v
		case []string:
			currentVal = strings.Join(v, ", ")
		}
		html.WriteString(fmt.Sprintf(`<form hx-post="%s" hx-trigger="%s" hx-swap="%s"%s>`, postURL, trigger, swap, targetAttr))
		html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, s.Key(), t(meta.Label)))
		html.WriteString(fmt.Sprintf(`<textarea name="%s" id="%s" class="form-textarea" placeholder="">%s</textarea>`, s.Key(), s.Key(), currentVal))
		if meta.Desc != "" {
			html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, t(meta.Desc)))
		}
		html.WriteString(`</form>`)
	}

	html.WriteString(`</div>`)
	return html.String()
}
