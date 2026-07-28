package settingstest

import (
	"fmt"
	"slices"

	"knov/internal/configmanager"
	"knov/internal/test"
	"knov/internal/thememanager"
)

// caseThemeList covers handleAPIGetThemes' GetAvailableThemes - "builtin" and "example" are both
// always-loaded real themes (themes/builtin, themes/example), so both must be present regardless
// of environment.
func caseThemeList() test.CaseResult {
	name := "theme-list"

	tm := thememanager.GetThemeManager()
	available := tm.GetAvailableThemes()
	names := make([]string, len(available))
	for i, t := range available {
		names[i] = t.Name
	}

	hasBuiltin := slices.Contains(names, "builtin")
	hasExample := slices.Contains(names, "example")

	success := hasBuiltin && hasExample
	cr := test.CaseResult{
		Name:     name,
		Expected: "available themes include builtin and example",
		Actual:   fmt.Sprintf("available=%v", names),
		Success:  success,
	}
	if !success {
		cr.Error = "GetAvailableThemes did not include the expected always-loaded themes"
	}
	return cr
}

// caseThemeSwitch covers handleAPISetTheme's GetAvailableThemes-lookup + SetCurrentTheme path,
// toggling between builtin/example and back - both are guaranteed present by caseThemeList.
func caseThemeSwitch() test.CaseResult {
	name := "theme-switch"

	tm := thememanager.GetThemeManager()
	original := tm.GetCurrentThemeName()
	defer configmanager.SetTheme(original)

	target := "example"
	if original == "example" {
		target = "builtin"
	}

	var targetTheme thememanager.Theme
	for _, t := range tm.GetAvailableThemes() {
		if t.Name == target {
			targetTheme = t
			break
		}
	}
	if targetTheme.Name == "" {
		return errCase(name, fmt.Errorf("target theme %q not found among available themes", target))
	}

	if err := tm.SetCurrentTheme(targetTheme); err != nil {
		return errCase(name, err)
	}

	tmAfter := thememanager.GetThemeManager()
	switched := tmAfter.GetCurrentThemeName() == target

	success := switched
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("current theme becomes %q", target),
		Actual:   fmt.Sprintf("current theme=%q", tmAfter.GetCurrentThemeName()),
		Success:  success,
	}
	if !success {
		cr.Error = "SetCurrentTheme did not switch the active theme as expected"
	}
	return cr
}

// caseThemeSettingsRoundtrip covers handleAPIGetThemeSettings/handleAPISetThemeSetting's
// SetThemeSetting/GetThemeSetting/GetCurrentThemeSettings path, using builtin's "darkMode"
// boolean setting (themes/builtin/theme.json) as a probe. Uses SetTheme rather than
// SetCurrentTheme so the probe applies regardless of whichever theme is active when the suite
// runs, without needing to switch the active theme.
func caseThemeSettingsRoundtrip() test.CaseResult {
	name := "theme-settings-roundtrip"

	const probeKey = "darkMode"
	original := configmanager.GetThemeSetting("builtin", probeKey)
	// origBool defaults to false when no override was ever stored (GetThemeSetting returns
	// nil) - restoreValue falls back to themes/builtin/theme.json's declared default (true)
	// instead, so a fresh install doesn't end up with a spurious explicit "false" override.
	origBool, existed := original.(bool)
	restoreValue := origBool
	if !existed {
		restoreValue = true
	}
	defer configmanager.SetThemeSetting("builtin", probeKey, restoreValue)

	probe := !restoreValue
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
