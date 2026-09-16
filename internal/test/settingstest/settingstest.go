// Package settingstest - Themes suite: exercises thememanager's theme list/switch against the
// real installed theme files on disk (configmanager.GetThemesPath()), which only exist against
// a real app instance - not something a sandboxed go test can cheaply fake. Every other former
// settingstest case (plain settings, favicon, hidePaths, languages, theme settings persistence)
// only needed configStorage and has moved to internal/configmanager/configmanager_settings_test.go.
//
// Every case here mutates real persisted global state (not sandboxed docs/test/ data), same
// category as exporttest's caseSettingsExportImportRoundtrip - each case captures the original
// value and restores it via defer.
package settingstest

import (
	"knov/internal/test"
)

// Suite runs the theme test cases against the real installed themes.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "settings" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseThemeList,
		caseThemeSwitch,
	}

	return test.RunCases("settings", cases), nil
}
