package settingstest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"knov/internal/configmanager"
	"knov/internal/test"
)

// caseLanguages covers handleAPIGetLanguages/handleAPISetSetting("language") - the available
// language list plus switching the active language and back.
func caseLanguages() test.CaseResult {
	name := "languages"

	available := configmanager.GetAvailableLanguages()
	codes := make([]string, len(available))
	for i, l := range available {
		codes[i] = l.Code
	}
	if !slices.Contains(codes, "en") || !slices.Contains(codes, "de") {
		return errCase(name, fmt.Errorf("expected languages en/de, got %v", codes))
	}

	original := configmanager.GetLanguage()
	defer configmanager.SetLanguage(original)

	target := "de"
	if original == "de" {
		target = "en"
	}
	configmanager.SetLanguage(target)

	switched := configmanager.GetLanguage() == target
	success := switched
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("available=%v, active language becomes %q", codes, target),
		Actual:   fmt.Sprintf("active=%q", configmanager.GetLanguage()),
		Success:  success,
	}
	if !success {
		cr.Error = "SetLanguage did not switch the active language as expected"
	}
	return cr
}

// caseFaviconUploadDelete replicates handleAPIUploadFavicon/handleAPIDeleteFavicon's file
// write/validate/delete sequence directly - both are inline handler logic with no exported
// wrapper beyond the GetCustomFaviconExt/SetCustomFaviconExt/GetCustomFaviconPath accessors.
// If a real custom favicon is already configured, its file is backed up and restored so the
// probe (which reuses the same "favicon.png" path when the original extension is also .png)
// never leaves the real favicon lost.
func caseFaviconUploadDelete() test.CaseResult {
	name := "favicon-upload-delete"

	const probeExt = ".png"
	faviconDir := filepath.Join(configmanager.GetAppConfig().StoragePath, "favicon")
	probePath := filepath.Join(faviconDir, "favicon"+probeExt)

	origExt := configmanager.GetCustomFaviconExt()
	var origData []byte
	if origExt == probeExt {
		if data, err := os.ReadFile(probePath); err == nil {
			origData = data
		}
	}
	defer func() {
		configmanager.SetCustomFaviconExt(origExt)
		if origData != nil {
			os.WriteFile(probePath, origData, 0644)
		}
	}()

	if err := os.MkdirAll(faviconDir, 0755); err != nil {
		return errCase(name, err)
	}
	probeBytes := []byte("settingstest favicon probe")
	if err := os.WriteFile(probePath, probeBytes, 0644); err != nil {
		return errCase(name, err)
	}
	configmanager.SetCustomFaviconExt(probeExt)

	uploadedExt := configmanager.GetCustomFaviconExt()
	uploadedPath := configmanager.GetCustomFaviconPath()
	uploaded := uploadedExt == probeExt && uploadedPath == probePath

	if err := os.Remove(probePath); err != nil {
		return errCase(name, err)
	}
	configmanager.SetCustomFaviconExt("")
	deleted := configmanager.GetCustomFaviconExt() == ""

	success := uploaded && deleted
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("upload sets ext=%q and path=%q, delete clears ext", probeExt, probePath),
		Actual:   fmt.Sprintf("uploaded=%v (ext=%q path=%q) deleted=%v", uploaded, uploadedExt, uploadedPath, deleted),
		Success:  success,
	}
	if !success {
		cr.Error = "favicon upload/delete sequence did not behave as expected"
	}
	return cr
}
