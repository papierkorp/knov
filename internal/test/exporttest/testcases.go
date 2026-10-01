package exporttest

import (
	"encoding/json"
	"fmt"
	"slices"

	"knov/internal/configmanager"
	"knov/internal/test"
)

// caseSettingsExportImportRoundtrip covers configmanager.ExportSettingsJSON/ImportSettingsJSON
// (internal/server/api_config.go's export/import handlers wrap these directly, no other
// inline logic to replicate) using HideTodo as a representative probe setting.
func caseSettingsExportImportRoundtrip() test.CaseResult {
	name := "settings-export-import-roundtrip"

	original := configmanager.HideTodo.Get()
	defer configmanager.SetSetting(configmanager.HideTodo, fmt.Sprintf("%v", original))

	probeValue := !original
	configmanager.SetSetting(configmanager.HideTodo, fmt.Sprintf("%v", probeValue))
	exported, err := configmanager.ExportSettingsJSON()
	if err != nil {
		return errCase(name, err)
	}

	configmanager.SetSetting(configmanager.HideTodo, fmt.Sprintf("%v", original))
	if _, err := configmanager.ImportSettingsJSON(exported); err != nil {
		return errCase(name, err)
	}

	restored := configmanager.HideTodo.Get()
	success := restored == probeValue
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("hideTodo=%v exported, changed to %v, import restores %v", probeValue, original, probeValue),
		Actual:   fmt.Sprintf("restored=%v", restored),
		Success:  success,
	}
	if !success {
		cr.Error = "ExportSettingsJSON/ImportSettingsJSON did not round-trip the setting as expected"
	}
	return cr
}

// caseSettingsImportReportsSkipped checks that ImportSettingsJSON reports a stored value that
// fails its setting's validation (here an out-of-range pageSize) back to the caller as skipped,
// instead of only logging it - handleAPIImportSettings (internal/server/api_config.go) relies on
// this to warn the user that part of their import didn't apply, rather than flashing an
// unqualified "imported successfully".
func caseSettingsImportReportsSkipped() test.CaseResult {
	name := "settings-import-reports-skipped"

	exported, err := configmanager.ExportSettingsJSON()
	if err != nil {
		return errCase(name, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(exported, &raw); err != nil {
		return errCase(name, err)
	}
	raw["pageSize"] = -1
	tampered, err := json.Marshal(raw)
	if err != nil {
		return errCase(name, err)
	}

	skipped, err := configmanager.ImportSettingsJSON(tampered)
	if err != nil {
		return errCase(name, err)
	}

	success := slices.Contains(skipped, "pageSize")
	cr := test.CaseResult{
		Name:     name,
		Expected: `skipped contains "pageSize"`,
		Actual:   fmt.Sprintf("skipped=%v", skipped),
		Success:  success,
	}
	if !success {
		cr.Error = "ImportSettingsJSON did not report the invalid pageSize value as skipped"
	}
	return cr
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
