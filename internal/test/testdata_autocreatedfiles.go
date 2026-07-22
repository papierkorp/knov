// Package testdata - auto-generated test files (testA/testB/testC structure) and their metadata
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

var autoTestFiles = []string{
	"test/testA/testA.md",
	"test/testA/testAB.md",
	"test/testA/testAC.md",
	"test/testA/testAA/testAAA.md",
	"test/testA/testAA/testAAB.md",
	"test/testA/testAA/testAAC.md",
	"test/testA/testAB/testABA.md",
	"test/testA/testAB/testABB.md",
	"test/testA/testAC/testACA.md",
	"test/testB/testB.md",
	"test/testB/testBA.md",
	"test/testB/testBB.md",
	"test/testB/testBC.md",
	"test/testC/testC.md",
	"test/testC/testCA.md",
	"test/testC/testCB.md",
	"test/testC/testCC.md",
}

// createTestStructure creates the auto-generated folder structure and files
func createTestStructure() error {
	logging.LogInfo(logging.KeyApp, "creating test structure")

	docsPath := contentStorage.GetDocsPath()

	dirs := []string{
		"test/testA/testAA",
		"test/testA/testAB",
		"test/testA/testAC",
		"test/testB",
		"test/testC",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(docsPath, dir), 0755); err != nil {
			return err
		}
	}

	return setAutoTestFileContent()
}

func setAutoTestFileContent() error {
	docsPath := contentStorage.GetDocsPath()

	for i, file := range autoTestFiles {
		fullPath := filepath.Join(docsPath, file)

		content := "# " + filepath.Base(file) + "\n\n**this is a test file.**\n\n"

		link1Idx := (i + 3) % len(autoTestFiles)
		link2Idx := (i + 7) % len(autoTestFiles)

		if link1Idx == i {
			link1Idx = (i + 1) % len(autoTestFiles)
		}
		if link2Idx == i {
			link2Idx = (i + 2) % len(autoTestFiles)
		}

		content += fmt.Sprintf("- [%s](%s)\n", autoTestFiles[link1Idx], autoTestFiles[link1Idx])
		content += fmt.Sprintf("- [%s](%s)\n", autoTestFiles[link2Idx], autoTestFiles[link2Idx])

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func simulateFileChange() error {
	logging.LogInfo(logging.KeyApp, "simulating file changes for version history")

	changedFiles := []string{
		"test/testA/testA.md",
		"test/testA/testAB.md",
		"test/testA/testAC.md",
		"test/testA/testAA/testAAA.md",
		"test/testA/testAA/testAAB.md",
		"test/testA/testAA/testAAC.md",
		"test/testA/testAB/testABA.md",
		"test/testA/testAB/testABB.md",
		"test/testA/testAC/testACA.md",
		"test/testB/testB.md",
		"test/testB/testBA.md",
		"test/testB/testBB.md",
		"test/testB/testBC.md",
		"test/testC/testC.md",
		"test/testC/testCA.md",
		"test/testC/testCB.md",
		"test/testC/testCC.md",
	}

	for _, file := range changedFiles {
		fullPath := pathutils.ToDocsPath(file)
		if content, err := os.ReadFile(fullPath); err == nil {
			updated := string(content) + "\n\n## Additional content\n- test for git version history\n- A single list element looks too empty"
			if err := os.WriteFile(fullPath, []byte(updated), 0644); err != nil {
				continue
			}
		}
	}

	if err := commitGitChanges("update test files with additional content"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to commit test file changes: %v", err)
	}

	return nil
}

// createAutoMetadata creates metadata for the auto-generated test files
func createAutoMetadata() error {
	// build skip set from copied files so parent logic ignores them
	skipPaths := map[string]bool{}
	for _, m := range getCopiedFilesMetadata() {
		skipPaths[m.Path] = true
	}

	for i, file := range autoTestFiles {
		absPath := filepath.Join(contentStorage.GetDocsPath(), file)
		relPath := strings.TrimPrefix(pathutils.ToRelative(absPath), "docs/")
		metadataPath := filepath.Join("docs", relPath)

		if skipPaths[metadataPath] {
			continue
		}

		folders := strings.Split(filepath.Dir(relPath), string(filepath.Separator))
		var validFolders []string
		for _, f := range folders {
			if f != "" && f != "." {
				validFolders = append(validFolders, f)
			}
		}

		var parents []string
		if i > 0 {
			parentCount := i % 3
			if parentCount > 2 {
				parentCount = 2
			}
			for j := 0; j < parentCount && j < i; j++ {
				parentIdx := i - 1 - (j * 2)
				if parentIdx >= 0 && parentIdx < i {
					parentAbs := filepath.Join(contentStorage.GetDocsPath(), autoTestFiles[parentIdx])
					parentPath := pathutils.ToWithPrefix(pathutils.ToRelative(parentAbs))
					if parentPath != metadataPath && !contains(parents, parentPath) {
						parents = append(parents, parentPath)
					}
				}
			}
		}

		metadata := &files.Metadata{
			Path:    metadataPath,
			Tags:    extractFilenameTags(filepath.Base(file)),
			Parents: parents,
			Editor:  files.EditorTypeToastUI,
		}

		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError(logging.KeyApp, "failed to save test metadata for %s: %v", file, err)
		}
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func extractFilenameTags(filename string) []string {
	basename := strings.TrimSuffix(filename, ".md")

	if !strings.HasPrefix(basename, "test") {
		return []string{}
	}

	remaining := strings.TrimPrefix(basename, "test")
	seen := map[string]bool{}
	var tags []string

	for _, char := range remaining {
		if char >= 'A' && char <= 'Z' {
			letter := string(char)
			if !seen[letter] {
				seen[letter] = true
				tags = append(tags, "test"+letter)
			}
		}
	}

	tags = append(tags, "kb-status-inbox")

	return tags
}
