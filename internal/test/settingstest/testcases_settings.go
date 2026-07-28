package settingstest

import (
	"fmt"

	"knov/internal/configmanager"
	"knov/internal/test"
)

// caseIndividualSetSetting covers handleAPISetSetting's path: GetSetting(key).SetFromString +
// SaveSettings, for a single boolean setting.
func caseIndividualSetSetting() test.CaseResult {
	name := "individual-set-setting"

	setting := configmanager.GetSetting("spellCheck").(*configmanager.BoolSetting)
	original := setting.Get()
	defer func() {
		setting.SetFromString(fmt.Sprintf("%v", original))
		configmanager.SaveSettings()
	}()

	probe := !original
	if err := setting.SetFromString(fmt.Sprintf("%v", probe)); err != nil {
		return errCase(name, err)
	}
	if err := configmanager.SaveSettings(); err != nil {
		return errCase(name, err)
	}

	success := setting.Get() == probe
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("spellCheck=%v after SetFromString+SaveSettings", probe),
		Actual:   fmt.Sprintf("spellCheck=%v", setting.Get()),
		Success:  success,
	}
	if !success {
		cr.Error = "individual setting did not persist the new value"
	}
	return cr
}

// caseBulkSetSettings covers handleAPIBulkSetSettings' BulkSetFromForm applying multiple
// settings of different types in one call.
func caseBulkSetSettings() test.CaseResult {
	name := "bulk-set-settings"

	pageSize := configmanager.GetSetting("pageSize").(*configmanager.IntSetting)
	vimMode := configmanager.GetSetting("codeMirrorVimMode").(*configmanager.BoolSetting)
	origPageSize, origVimMode := pageSize.Get(), vimMode.Get()
	defer func() {
		pageSize.SetFromString(fmt.Sprintf("%d", origPageSize))
		vimMode.SetFromString(fmt.Sprintf("%v", origVimMode))
		configmanager.SaveSettings()
	}()

	probePageSize := origPageSize + 1
	probeVimMode := !origVimMode
	errs := configmanager.BulkSetFromForm(map[string][]string{
		"pageSize":          {fmt.Sprintf("%d", probePageSize)},
		"codeMirrorVimMode": {fmt.Sprintf("%v", probeVimMode)},
	})

	success := len(errs) == 0 && pageSize.Get() == probePageSize && vimMode.Get() == probeVimMode
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("pageSize=%d codeMirrorVimMode=%v applied in one call, no errors", probePageSize, probeVimMode),
		Actual:   fmt.Sprintf("pageSize=%d codeMirrorVimMode=%v errs=%v", pageSize.Get(), vimMode.Get(), errs),
		Success:  success,
	}
	if !success {
		cr.Error = "BulkSetFromForm did not apply both settings as expected"
	}
	return cr
}

// caseBulkSetUnknownKeySkipped covers BulkSetFromForm's partial-update semantics: only keys
// present in the submitted form are touched (an omitted setting keeps its prior value), and an
// unrecognised key is skipped silently rather than producing an error.
func caseBulkSetUnknownKeySkipped() test.CaseResult {
	name := "bulk-set-unknown-key-skipped"

	spellCheck := configmanager.GetSetting("spellCheck").(*configmanager.BoolSetting)
	vimMode := configmanager.GetSetting("codeMirrorVimMode").(*configmanager.BoolSetting)
	origSpellCheck, origVimMode := spellCheck.Get(), vimMode.Get()
	defer func() {
		spellCheck.SetFromString(fmt.Sprintf("%v", origSpellCheck))
		configmanager.SaveSettings()
	}()

	probeSpellCheck := !origSpellCheck
	errs := configmanager.BulkSetFromForm(map[string][]string{
		"spellCheck":               {fmt.Sprintf("%v", probeSpellCheck)},
		"totallyUnknownSetting123": {"whatever"},
	})

	appliedTouched := spellCheck.Get() == probeSpellCheck
	untouchedStayed := vimMode.Get() == origVimMode

	success := len(errs) == 0 && appliedTouched && untouchedStayed
	cr := test.CaseResult{
		Name:     name,
		Expected: "unknown key skipped without error, submitted key applied, omitted key left untouched",
		Actual:   fmt.Sprintf("errs=%v appliedTouched=%v untouchedStayed=%v", errs, appliedTouched, untouchedStayed),
		Success:  success,
	}
	if !success {
		cr.Error = "BulkSetFromForm did not honor partial-update semantics as expected"
	}
	return cr
}

// caseBulkSetValidationError covers a validation failure (pageSize is bounded 5-200) being
// returned as an error and the setting's value left unchanged.
func caseBulkSetValidationError() test.CaseResult {
	name := "bulk-set-validation-error"

	pageSize := configmanager.GetSetting("pageSize").(*configmanager.IntSetting)
	original := pageSize.Get()

	errs := configmanager.BulkSetFromForm(map[string][]string{"pageSize": {"100000"}})

	unchanged := pageSize.Get() == original
	success := len(errs) > 0 && unchanged
	cr := test.CaseResult{
		Name:     name,
		Expected: "out-of-range pageSize rejected with an error, value left unchanged",
		Actual:   fmt.Sprintf("errs=%v pageSize=%d (original %d)", errs, pageSize.Get(), original),
		Success:  success,
	}
	if !success {
		cr.Error = "BulkSetFromForm did not reject the invalid value as expected"
	}
	return cr
}
