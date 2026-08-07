package settingstest

import (
	"fmt"
	"slices"

	"knov/internal/configmanager"
	"knov/internal/test"
	"knov/internal/thememanager"
)

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

// caseThemeSettingsRoundtrip covers handleAPIGetThemeSettings/handleAPISetThemeSetting's
// SetThemeSetting/GetThemeSetting/GetCurrentThemeSettings path, using builtin's "colorScheme"
// select setting (themes/builtin/theme.json) as a probe. Uses SetTheme rather than
// SetCurrentTheme so the probe applies regardless of whichever theme is active when the suite
// runs, without needing to switch the active theme.
func caseThemeSettingsRoundtrip() test.CaseResult {
	name := "theme-settings-roundtrip"

	const probeKey = "colorScheme"
	original := configmanager.GetThemeSetting("builtin", probeKey)
	// origStr defaults to "" when no override was ever stored (GetThemeSetting returns nil) -
	// restoreValue falls back to themes/builtin/theme.json's declared default ("green")
	// instead, so a fresh install doesn't end up with a spurious explicit override.
	origStr, existed := original.(string)
	restoreValue := origStr
	if !existed {
		restoreValue = "green"
	}
	defer configmanager.SetThemeSetting("builtin", probeKey, restoreValue)

	probe := "blue"
	if restoreValue == "blue" {
		probe = "red"
	}
	configmanager.SetThemeSetting("builtin", probeKey, probe)

	got := configmanager.GetThemeSetting("builtin", probeKey)
	current := configmanager.GetCurrentThemeSettings()

	gotMatches := got == probe
	inCurrentMap := false
	if configmanager.GetTheme() == "builtin" {
		v, ok := current[probeKey]
		inCurrentMap = ok && v == probe
	} else {
		inCurrentMap = true // GetCurrentThemeSettings reflects the active theme, not builtin - not applicable here
	}

	success := gotMatches && inCurrentMap
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("builtin.%s=%v after SetThemeSetting", probeKey, probe),
		Actual:   fmt.Sprintf("GetThemeSetting=%v", got),
		Success:  success,
	}
	if !success {
		cr.Error = "theme setting did not round-trip through SetThemeSetting/GetThemeSetting as expected"
	}
	return cr
}
