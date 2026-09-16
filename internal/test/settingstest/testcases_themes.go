package settingstest

import (
	"fmt"
	"slices"

	"knov/internal/configmanager"
	"knov/internal/test"
	"knov/internal/thememanager"
)

// caseThemeList and caseThemeSwitch are the two cases left in this suite: they depend on the
// real installed theme files on disk (thememanager.GetThemeManager loads themes/<name>/theme.json
// from configmanager.GetThemesPath()), which only exist against a real app instance - not
// something a sandboxed go test can cheaply fake. Every other former settingstest/configtest
// case (plain settings, favicon, hidePaths, languages, theme *settings* persistence, which only
// needs configStorage) has moved to internal/configmanager/configmanager_settings_test.go.

// caseThemeList covers handleAPIGetThemes' GetAvailableThemes - "builtin" (themes/builtin) is the
// only theme guaranteed to ship, so it's the only one asserted here. Any other themes are
// environment-specific (dev fixtures, user-installed) and not assumed present.
func caseThemeList() test.CaseResult {
	name := "theme-list"

	tm := thememanager.GetThemeManager()
	available := tm.GetAvailableThemes()
	names := make([]string, len(available))
	for i, t := range available {
		names[i] = t.Name
	}

	success := slices.Contains(names, "builtin")
	cr := test.CaseResult{
		Name:     name,
		Expected: "available themes include builtin",
		Actual:   fmt.Sprintf("available=%v", names),
		Success:  success,
	}
	if !success {
		cr.Error = "GetAvailableThemes did not include the always-loaded builtin theme"
	}
	return cr
}

// caseThemeSwitch covers handleAPISetTheme's GetAvailableThemes-lookup + SetCurrentTheme path,
// switching to whatever non-current theme happens to be installed. Only "builtin" is guaranteed
// to ship, so if it's the only theme available there's nothing to switch to and the case passes
// trivially instead of assuming a second theme (e.g. "example") exists.
func caseThemeSwitch() test.CaseResult {
	name := "theme-switch"

	tm := thememanager.GetThemeManager()
	original := tm.GetCurrentThemeName()
	defer configmanager.SetTheme(original)

	var target thememanager.Theme
	for _, t := range tm.GetAvailableThemes() {
		if t.Name != original {
			target = t
			break
		}
	}
	if target.Name == "" {
		return test.CaseResult{
			Name:     name,
			Expected: "current theme switches to another installed theme",
			Actual:   "only one theme installed, nothing to switch to",
			Success:  true,
		}
	}

	if err := tm.SetCurrentTheme(target); err != nil {
		return errCase(name, err)
	}

	tmAfter := thememanager.GetThemeManager()
	switched := tmAfter.GetCurrentThemeName() == target.Name

	success := switched
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("current theme becomes %q", target.Name),
		Actual:   fmt.Sprintf("current theme=%q", tmAfter.GetCurrentThemeName()),
		Success:  success,
	}
	if !success {
		cr.Error = "SetCurrentTheme did not switch the active theme as expected"
	}
	return cr
}
