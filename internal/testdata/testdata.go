// Package testdata - Test data setup and management
package testdata

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
)

// SetupTestData creates test files and git operations
func SetupTestData() error {
	if err := copyTestFiles(); err != nil {
		return err
	}

	if err := createGitOperations(); err != nil {
		return err
	}

	if err := setupTestMetadata(); err != nil {
		return err
	}

	logging.LogInfo("test data setup completed")
	return nil
}

// CleanTestData removes all test data
func CleanTestData() error {
	dataPath := configmanager.GetAppConfig().DataPath
	if err := os.RemoveAll(dataPath); err != nil {
		logging.LogError("failed to remove data directory: %v", err)
		return err
	}

	if err := os.RemoveAll("config/.metadata"); err != nil {
		logging.LogError("failed to remove metadata directory: %v", err)
		return err
	}

	logging.LogInfo("test data cleaned")
	return nil
}

func copyTestFiles() error {
	logging.LogInfo("copying test files")

	dataPath := configmanager.GetAppConfig().DataPath
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return err
	}

	cmd := exec.Command("cp", "-r", "internal/testdata/testfiles/.", dataPath+"/")
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to copy test files: %v", err)
		return err
	}

	return nil
}

func createTestStructure() error {
	logging.LogInfo("creating test structure")

	dataPath := configmanager.GetAppConfig().DataPath

	dirs := []string{
		"test/testA/testAA",
		"test/testA/testAB",
		"test/testA/testAC",
		"test/testB",
		"test/testC",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(dataPath, dir), 0755); err != nil {
			return err
		}
	}

	testFiles := []string{
		"test/testA/testA.md",
		"test/testA/testAB.md",
		"test/testA/testAC.md",
		"test/testA/testAA/testAAA.md",
		"test/testA/testAA/testAAB.md",
		"test/testA/testAA/testAAC.md",
		"test/testA/testAB/testABA.md",
		"test/testA/testAB/testABB.md",
		"test/testA/testAC/testACA.md",
		"test/testB/testBA.md",
		"test/testB/testBB.md",
		"test/testB/testBC.md",
		"test/testC/testCA.md",
		"test/testC/testCB.md",
		"test/testC/testCC.md",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(dataPath, file)
		content := "# " + filepath.Base(file) + "\n\nThis is a test file."
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func createGitOperations() error {
	logging.LogInfo("creating git operations")

	// Use the configmanager git initialization instead of manual git init
	if err := configmanager.InitGitRepository(); err != nil {
		return err
	}

	dataDir := configmanager.GetAppConfig().DataPath

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dataDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial test documentation", "--allow-empty")
	cmd.Dir = dataDir
	cmd.Run()

	// Simulate file change for git history
	gettingStartedPath := filepath.Join(dataDir, "getting-started.md")
	if content, err := os.ReadFile(gettingStartedPath); err == nil {
		updatedContent := string(content) + "\n\n## Recent Updates\n- Added troubleshooting section\n- Improved navigation"
		os.WriteFile(gettingStartedPath, []byte(updatedContent), 0644)

		cmd = exec.Command("git", "add", "getting-started.md")
		cmd.Dir = dataDir
		cmd.Run()

		cmd = exec.Command("git", "commit", "-m", "update getting started guide")
		cmd.Dir = dataDir
		cmd.Run()
	}

	if err := createTestStructure(); err != nil {
		return err
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dataDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add test structure")
	cmd.Dir = dataDir
	cmd.Run()

	return nil
}

func setupTestMetadata() error {
	logging.LogInfo("creating test metadata")

	defaultFiles := getDefaultFiles()
	for _, meta := range defaultFiles {
		if err := files.MetaDataSave(meta); err != nil {
			logging.LogError("failed to save metadata for %s: %v", meta.Path, err)
		}
	}

	if err := createTestMetadata(); err != nil {
		return err
	}

	return files.MetaDataLinksRebuild()
}

func createTestMetadata() error {
	var testFiles []string
	dataPath := configmanager.GetAppConfig().DataPath
	testDir := filepath.Join(dataPath, "test")

	err := filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			testFiles = append(testFiles, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	statuses := []files.Status{files.StatusDraft, files.StatusPublished, files.StatusPublished, files.StatusDraft}
	priorities := []files.Priority{files.PriorityLow, files.PriorityMedium, files.PriorityHigh}

	for i, file := range testFiles {
		filename := filepath.Base(file)
		relPath := strings.TrimPrefix(file, dataPath+"/")
		folders := strings.Split(filepath.Dir(relPath), "/")

		validFolders := []string{}
		for _, folder := range folders {
			if folder != "" && folder != "." {
				validFolders = append(validFolders, folder)
			}
		}

		collection := "default"
		if len(validFolders) > 0 && validFolders[0] != "" {
			collection = validFolders[0]
		}

		createDay := 3 + (i % 13)
		editDay := createDay + 3 + (i % 8)
		status := statuses[i%len(statuses)]
		priority := priorities[i%len(priorities)]

		tags := extractFilenameTags(filename)

		metadata := &files.Metadata{
			Name:       filename,
			Path:       file,
			CreatedAt:  time.Date(2025, 9, createDay, 8+(i%8), (i*7)%60, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, editDay, 10+(i%6), (i*13)%60, 0, 0, time.UTC),
			Collection: collection,
			Folders:    validFolders,
			Tags:       tags,
			Boards:     []string{},
			FileType:   files.FileTypeNote,
			Status:     status,
			Priority:   priority,
		}

		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError("failed to save test metadata for %s: %v", file, err)
		}
	}

	return nil
}

func extractFilenameTags(filename string) []string {
	basename := strings.TrimSuffix(filename, ".md")

	if !strings.HasPrefix(basename, "test") {
		return []string{}
	}

	remaining := strings.TrimPrefix(basename, "test")
	uniqueLetters := make(map[string]bool)
	var tags []string

	for _, char := range remaining {
		if char >= 'A' && char <= 'Z' {
			letter := string(char)
			if !uniqueLetters[letter] {
				uniqueLetters[letter] = true
				tags = append(tags, "test"+letter)
			}
		}
	}

	return tags
}
