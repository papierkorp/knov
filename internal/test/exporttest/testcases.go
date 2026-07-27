package exporttest

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dokuwikiconverter"
	"knov/internal/test"
)

// buildExportZip replicates handleAPIExportAllFiles/handleAPIExportAllFilesWithMarkdownConversion's
// walk+zip logic (internal/server/api_files.go) directly - both are inline handler logic with
// no exported wrapper function. convertMarkdown mirrors the markdown-conversion variant's
// .dokuwiki/.txt -> .md rename+conversion step; the raw variant passes false and zips every
// file unmodified.
func buildExportZip(convertMarkdown bool) (map[string][]byte, error) {
	dataPath := configmanager.GetAppConfig().DataPath

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	err := filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dataPath, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if convertMarkdown {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".dokuwiki" || ext == ".txt" {
				markdown := dokuwikiconverter.NewWithFilePath(relPath).ConvertToMarkdown(string(content))
				content = []byte(markdown)
				relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".md"
			}
		}

		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}
		_, err = zipFile.Write(content)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return nil, err
	}

	entries := make(map[string][]byte, len(reader.File))
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		entries[filepath.ToSlash(f.Name)] = data
	}
	return entries, nil
}

func caseExportZipRaw() test.CaseResult {
	name := "export-zip-raw"

	entries, err := buildExportZip(false)
	if err != nil {
		return errCase(name, err)
	}

	sampleEntry := "docs/" + testPath(sampleFile)
	txtEntry := "docs/" + testPath(txtFile)

	sampleOK := string(entries[sampleEntry]) == sampleContent
	txtOK := string(entries[txtEntry]) == dokuwikiContent

	success := sampleOK && txtOK
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%s and %s zipped unmodified (no markdown conversion)", sampleEntry, txtEntry),
		Actual:   fmt.Sprintf("sampleOK=%v txtOK=%v (%d entries)", sampleOK, txtOK, len(entries)),
		Success:  success,
	}
	if !success {
		cr.Error = "raw zip export did not include the sample files unmodified as expected"
	}
	return cr
}

func caseExportZipMarkdownConverted() test.CaseResult {
	name := "export-zip-markdown-converted"

	entries, err := buildExportZip(true)
	if err != nil {
		return errCase(name, err)
	}

	sampleEntry := "docs/" + testPath(sampleFile)
	oldTxtEntry := "docs/" + testPath(txtFile)
	newMdEntry := strings.TrimSuffix(oldTxtEntry, filepath.Ext(oldTxtEntry)) + ".md"

	sampleUnchanged := string(entries[sampleEntry]) == sampleContent
	_, oldStillPresent := entries[oldTxtEntry]
	converted := strings.Contains(string(entries[newMdEntry]), "# Heading") && strings.Contains(string(entries[newMdEntry]), "**bold text**")

	success := sampleUnchanged && !oldStillPresent && converted
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%s unchanged, %s renamed to %s and converted to markdown", sampleEntry, oldTxtEntry, newMdEntry),
		Actual:   fmt.Sprintf("sampleUnchanged=%v oldStillPresent=%v converted=%v", sampleUnchanged, oldStillPresent, converted),
		Success:  success,
	}
	if !success {
		cr.Error = "markdown-converted zip export did not rename/convert the .txt file as expected"
	}
	return cr
}

// caseSettingsExportImportRoundtrip covers configmanager.ExportSettingsJSON/ImportSettingsJSON
// (internal/server/api_config.go's export/import handlers wrap these directly, no other
// inline logic to replicate) using HideTodo as a representative probe setting.
func caseSettingsExportImportRoundtrip() test.CaseResult {
	name := "settings-export-import-roundtrip"

	original := configmanager.HideTodo.Get()
	defer configmanager.HideTodo.SetFromString(fmt.Sprintf("%v", original))

	probeValue := !original
	configmanager.HideTodo.SetFromString(fmt.Sprintf("%v", probeValue))
	exported, err := configmanager.ExportSettingsJSON()
	if err != nil {
		return errCase(name, err)
	}

	configmanager.HideTodo.SetFromString(fmt.Sprintf("%v", original))
	if err := configmanager.ImportSettingsJSON(exported); err != nil {
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
