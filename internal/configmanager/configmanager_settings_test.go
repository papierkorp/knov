package configmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"knov/internal/configStorage"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-configmanager-test")
	if err != nil {
		panic(err)
	}
	SetDataAndStoragePaths(dir, dir)
	if err := configStorage.Init("json", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// handleAPISetSetting's path: GetSetting(key).SetFromString + SaveSettings, for a single
// boolean setting.
func TestIndividualSetSetting(t *testing.T) {
	setting := GetSetting("spellCheck").(*BoolSetting)
	original := setting.Get()
	defer func() {
		setting.SetFromString(fmt.Sprintf("%v", original))
		SaveSettings()
	}()

	probe := !original
	if err := setting.SetFromString(fmt.Sprintf("%v", probe)); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(); err != nil {
		t.Fatal(err)
	}
	if setting.Get() != probe {
		t.Errorf("spellCheck = %v after SetFromString+SaveSettings, want %v", setting.Get(), probe)
	}
}

// handleAPIBulkSetSettings' BulkSetFromForm applying multiple settings of different types in
// one call.
func TestBulkSetSettings(t *testing.T) {
	pageSize := GetSetting("pageSize").(*IntSetting)
	vimMode := GetSetting("codeMirrorVimMode").(*BoolSetting)
	origPageSize, origVimMode := pageSize.Get(), vimMode.Get()
	defer func() {
		pageSize.SetFromString(fmt.Sprintf("%d", origPageSize))
		vimMode.SetFromString(fmt.Sprintf("%v", origVimMode))
		SaveSettings()
	}()

	probePageSize := origPageSize + 1
	probeVimMode := !origVimMode
	errs := BulkSetFromForm(map[string][]string{
		"pageSize":          {fmt.Sprintf("%d", probePageSize)},
		"codeMirrorVimMode": {fmt.Sprintf("%v", probeVimMode)},
	})

	if len(errs) != 0 {
		t.Errorf("BulkSetFromForm returned errors: %v", errs)
	}
	if pageSize.Get() != probePageSize {
		t.Errorf("pageSize = %d, want %d", pageSize.Get(), probePageSize)
	}
	if vimMode.Get() != probeVimMode {
		t.Errorf("codeMirrorVimMode = %v, want %v", vimMode.Get(), probeVimMode)
	}
}

// BulkSetFromForm's partial-update semantics: only keys present in the submitted form are
// touched (an omitted setting keeps its prior value), and an unrecognised key is skipped
// silently rather than producing an error.
func TestBulkSetUnknownKeySkipped(t *testing.T) {
	spellCheck := GetSetting("spellCheck").(*BoolSetting)
	vimMode := GetSetting("codeMirrorVimMode").(*BoolSetting)
	origSpellCheck, origVimMode := spellCheck.Get(), vimMode.Get()
	defer func() {
		spellCheck.SetFromString(fmt.Sprintf("%v", origSpellCheck))
		SaveSettings()
	}()

	probeSpellCheck := !origSpellCheck
	errs := BulkSetFromForm(map[string][]string{
		"spellCheck":               {fmt.Sprintf("%v", probeSpellCheck)},
		"totallyUnknownSetting123": {"whatever"},
	})

	if len(errs) != 0 {
		t.Errorf("BulkSetFromForm returned errors for an unknown key: %v", errs)
	}
	if spellCheck.Get() != probeSpellCheck {
		t.Error("submitted key was not applied")
	}
	if vimMode.Get() != origVimMode {
		t.Error("omitted key was touched")
	}
}

// a validation failure (pageSize is bounded 5-200) is returned as an error and the setting's
// value is left unchanged.
func TestBulkSetValidationError(t *testing.T) {
	pageSize := GetSetting("pageSize").(*IntSetting)
	original := pageSize.Get()

	errs := BulkSetFromForm(map[string][]string{"pageSize": {"100000"}})

	if len(errs) == 0 {
		t.Error("out-of-range pageSize was not rejected")
	}
	if pageSize.Get() != original {
		t.Errorf("pageSize = %d after rejected update, want unchanged %d", pageSize.Get(), original)
	}
}

// handleAPIGetLanguages/handleAPISetSetting("language") - the available language list plus
// switching the active language and back.
func TestLanguages(t *testing.T) {
	available := GetAvailableLanguages()
	codes := make([]string, len(available))
	for i, l := range available {
		codes[i] = l.Code
	}
	if !slices.Contains(codes, "en") || !slices.Contains(codes, "de") {
		t.Fatalf("expected languages en/de, got %v", codes)
	}

	original := GetLanguage()
	defer SetLanguage(original)

	target := "de"
	if original == "de" {
		target = "en"
	}
	SetLanguage(target)

	if GetLanguage() != target {
		t.Errorf("active language = %q, want %q", GetLanguage(), target)
	}
}

// handleAPISetSetting("hidePaths") rejects a HidePaths entry with an unrecognized "::tag" scope
// (e.g. a typo like "::serach") instead of silently accepting it as a pattern that's hidden
// everywhere.
func TestHidePathsTagValidation(t *testing.T) {
	prev := HidePaths.Get()
	defer HidePaths.SetFromString(strings.Join(prev, ","))

	err := HidePaths.SetFromString("archive::serach")
	if err == nil {
		t.Error("SetFromString(\"archive::serach\") did not return an error")
	}
	if !slices.Equal(HidePaths.Get(), prev) {
		t.Errorf("HidePaths changed to %v despite the rejected entry, want unchanged %v", HidePaths.Get(), prev)
	}
}

// IsTagHidden: "*" is a wildcard for any run of characters, case-insensitive; without
// a "*" a pattern only matches that exact tag. A "::scope" suffix limits the pattern to
// that scope, same as HidePaths.
func TestIsTagHidden(t *testing.T) {
	prev := HideTags.Get()
	defer HideTags.SetFromString(strings.Join(prev, ","))

	HideTags.SetFromString("kb-status*")
	patterns := HideTagsPatterns(HideScopeKanban)
	if !IsTagHidden(patterns, "kb-status-inbox") {
		t.Error(`"kb-status*" should hide "kb-status-inbox"`)
	}
	if !IsTagHidden(patterns, "KB-STATUS-INBOX") {
		t.Error(`"kb-status*" should hide "KB-STATUS-INBOX" (case-insensitive)`)
	}
	if IsTagHidden(patterns, "other-tag") {
		t.Error(`"kb-status*" should not hide "other-tag"`)
	}

	HideTags.SetFromString("kb-status")
	patterns = HideTagsPatterns(HideScopeKanban)
	if IsTagHidden(patterns, "kb-status-inbox") {
		t.Error(`"kb-status" without a wildcard should not hide "kb-status-inbox"`)
	}
	if !IsTagHidden(patterns, "kb-status") {
		t.Error(`"kb-status" should hide the exact tag "kb-status"`)
	}

	HideTags.SetFromString("kb-status*::kanban")
	if !IsTagHidden(HideTagsPatterns(HideScopeKanban), "kb-status-inbox") {
		t.Error(`"kb-status*::kanban" should hide "kb-status-inbox" in the kanban scope`)
	}
	if IsTagHidden(HideTagsPatterns(HideScopeDetail), "kb-status-inbox") {
		t.Error(`"kb-status*::kanban" should not hide "kb-status-inbox" in the detail scope`)
	}

	if err := HideTags.SetFromString("kb-status*::bogus"); err == nil {
		t.Error(`SetFromString("kb-status*::bogus") did not return an error`)
	}
}

// replicates handleAPIUploadFavicon/handleAPIDeleteFavicon's file write/validate/delete
// sequence directly - both are inline handler logic with no exported wrapper beyond the
// GetCustomFaviconExt/SetCustomFaviconExt/GetCustomFaviconPath accessors.
func TestFaviconUploadDelete(t *testing.T) {
	const probeExt = ".png"
	faviconDir := filepath.Join(GetAppConfig().StoragePath, "favicon")
	probePath := filepath.Join(faviconDir, "favicon"+probeExt)

	origExt := GetCustomFaviconExt()
	defer SetCustomFaviconExt(origExt)

	if err := os.MkdirAll(faviconDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(probePath, []byte("configmanager test favicon probe"), 0644); err != nil {
		t.Fatal(err)
	}
	SetCustomFaviconExt(probeExt)

	if got := GetCustomFaviconExt(); got != probeExt {
		t.Errorf("GetCustomFaviconExt = %q, want %q", got, probeExt)
	}
	if got := GetCustomFaviconPath(); got != probePath {
		t.Errorf("GetCustomFaviconPath = %q, want %q", got, probePath)
	}

	if err := os.Remove(probePath); err != nil {
		t.Fatal(err)
	}
	SetCustomFaviconExt("")
	if got := GetCustomFaviconExt(); got != "" {
		t.Errorf("GetCustomFaviconExt = %q after delete, want empty", got)
	}
}

// handleAPIGetThemeSettings/handleAPISetThemeSetting's SetThemeSetting/GetThemeSetting/
// GetCurrentThemeSettings path, using builtin's "colorScheme" select setting
// (themes/builtin/theme.json) as a probe.
func TestThemeSettingsRoundtrip(t *testing.T) {
	const probeKey = "colorScheme"
	original := GetThemeSetting("builtin", probeKey)
	// origStr defaults to "" when no override was ever stored (GetThemeSetting returns nil) -
	// restoreValue falls back to themes/builtin/theme.json's declared default ("green") instead,
	// so a fresh install doesn't end up with a spurious explicit override.
	origStr, existed := original.(string)
	restoreValue := origStr
	if !existed {
		restoreValue = "green"
	}
	defer SetThemeSetting("builtin", probeKey, restoreValue)

	probe := "blue"
	if restoreValue == "blue" {
		probe = "red"
	}
	SetThemeSetting("builtin", probeKey, probe)

	if got := GetThemeSetting("builtin", probeKey); got != probe {
		t.Errorf("GetThemeSetting = %v, want %v", got, probe)
	}
	if GetTheme() == "builtin" {
		if v, ok := GetCurrentThemeSettings()[probeKey]; !ok || v != probe {
			t.Errorf("GetCurrentThemeSettings()[%q] = %v, want %v", probeKey, v, probe)
		}
	}
}
