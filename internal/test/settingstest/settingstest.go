// Package settingstest - Settings/themes/config suite: exercises configmanager's exported
// settings registry (bulk + individual set, validation, partial-update semantics),
// thememanager's theme list/switch/settings, and configmanager's language/favicon accessors
// directly - every function involved here is exported (internal/server/api_settings.go,
// api_themes.go and api_config.go are thin wrappers with no business logic to replicate,
// except favicon upload/delete which inlines its file write in the handler).
//
// Every case here mutates real persisted global settings (not sandboxed docs/test/ data),
// same category as exporttest's caseSettingsExportImportRoundtrip - each case captures the
// original value and restores it via defer.
//
// Config repo url (handleAPISetGitRepositoryURL) is not covered here - it's the exact same
// configmanager.UpdateEnvFile("KNOV_GIT_REMOTE", ...) + git.EnsureRemote() pair githistorytest's
// caseGitRemotePushPullTestAuth already exercises (with its own restore). Config data path
// change is out of scope entirely, per step 6's note: it only writes .env and needs a process
// restart (os.Exit(0)) to take effect, which an in-app suite can't safely trigger+reverify.
package settingstest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the settings/themes/config test cases against the real settings store.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("settings-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "settings" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseIndividualSetSetting,
		caseBulkSetSettings,
		caseBulkSetUnknownKeySkipped,
		caseBulkSetValidationError,
		caseThemeList,
		caseThemeSwitch,
		caseThemeSettingsRoundtrip,
		caseLanguages,
		caseFaviconUploadDelete,
	}

	result := &test.SuiteResult{Suite: "settings"}
	for _, c := range cases {
		cr := c()
		result.Cases = append(result.Cases, cr)
		if cr.Success {
			result.Passed++
		} else {
			result.Failed++
		}
	}
	result.Total = len(cases)
	result.Success = result.Failed == 0
	return result, nil
}
