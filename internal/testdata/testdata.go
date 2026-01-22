// Package testdata - Test data setup and management
package testdata

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pathutils"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var testFilesFS embed.FS

// SetTestFiles sets the embedded test files
func SetTestFiles(filesFS embed.FS) {
	testFilesFS = filesFS
}

// SetupTestData creates test files and git operations
func SetupTestData() error {
	if err := copyTestFiles(); err != nil {
		return err
	}

	if err := createGitOperations("initial test documentation"); err != nil {
		return err
	}

	if err := setupTestMetadata(); err != nil {
		return err
	}

	if err := simulateFileChange(); err != nil {
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

	logging.LogInfo("test data cleaned - removed data directory: %s", dataPath)
	return nil
}

func copyTestFiles() error {
	logging.LogInfo("copying test files")

	docsPath := contentStorage.GetDocsPath()
	if err := os.MkdirAll(docsPath, 0755); err != nil {
		return err
	}

	srcDir := "internal/testdata/testfiles"

	return fs.WalkDir(testFilesFS, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == srcDir {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(docsPath, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// ensure parent directory exists before copying file
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// read from embedded FS and write to destination
		data, err := testFilesFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

func createTestStructure() error {
	logging.LogInfo("creating test structure")

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

	return setTestDataContent()
}

func setTestDataContent() error {
	docsPath := contentStorage.GetDocsPath()

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
		"test/testB/testB.md",
		"test/testB/testBA.md",
		"test/testB/testBB.md",
		"test/testB/testBC.md",
		"test/testC/testC.md",
		"test/testC/testCA.md",
		"test/testC/testCB.md",
		"test/testC/testCC.md",
	}

	for i, file := range testFiles {
		fullPath := filepath.Join(docsPath, file)

		// create content with links
		content := "# " + filepath.Base(file) + "\n\n**This is a test file.**\n\n"

		// add 2 pseudo-random links to other test files
		link1Idx := (i + 3) % len(testFiles)
		link2Idx := (i + 7) % len(testFiles)

		// ensure we don't link to ourselves
		if link1Idx == i {
			link1Idx = (i + 1) % len(testFiles)
		}
		if link2Idx == i {
			link2Idx = (i + 2) % len(testFiles)
		}

		content += fmt.Sprintf("- [%s](%s)\n", testFiles[link1Idx], testFiles[link1Idx])
		content += fmt.Sprintf("- [%s](%s)\n", testFiles[link2Idx], testFiles[link2Idx])

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func createGitOperations(commitMessage string) error {
	logging.LogInfo("creating git operations")

	// commit initial files
	if err := commitGitChanges(commitMessage); err != nil {
		return err
	}

	// create test structure
	if err := createTestStructure(); err != nil {
		return err
	}

	// commit test structure
	if err := commitGitChanges("add test structure"); err != nil {
		return err
	}

	return nil
}

func commitGitChanges(commitMessage string) error {
	// use the configmanager git initialization instead of manual git init
	if err := configmanager.InitGitRepository(); err != nil {
		return err
	}

	dataDir := configmanager.GetAppConfig().DataPath
	repo, err := git.PlainOpen(dataDir)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	// add all files
	_, err = worktree.Add(".")
	if err != nil {
		logging.LogError("failed to add files: %v", err)
	}

	// commit with provided message
	_, err = worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
		AllowEmptyCommits: true,
	})
	if err != nil {
		logging.LogError("failed to commit: %v", err)
	}

	return nil
}

func simulateFileChange() error {
	logging.LogInfo("simulating file changes for version history")

	// simulate file change for git history on getting-started.md
	gettingStartedPath := pathutils.ToDocsPath("getting-started.md")
	if content, err := os.ReadFile(gettingStartedPath); err == nil {
		updatedContent := string(content) + "\n\n## Recent Updates\n- Added troubleshooting section\n- Improved navigation"
		os.WriteFile(gettingStartedPath, []byte(updatedContent), 0644)
	}

	// create git commit for getting-started.md
	if err := commitGitChanges("update getting started guide"); err != nil {
		logging.LogWarning("failed to commit getting-started.md changes: %v", err)
	}

	// simulate changes on all test files to create multiple versions
	testFiles := []string{
		"test/testB/testB.md",
		"test/testB/testBA.md",
		"test/testB/testBB.md",
		"test/testB/testBC.md",
	}

	for _, file := range testFiles {
		fullPath := pathutils.ToDocsPath(file)
		if content, err := os.ReadFile(fullPath); err == nil {
			updatedContent := string(content) + "\n\n## Version Update\n- Added documentation section\n- Enhanced content structure"
			if err := os.WriteFile(fullPath, []byte(updatedContent), 0644); err != nil {
				continue // skip files that can't be written
			}
		}
	}

	// create git commit for test files
	if err := commitGitChanges("update test files with additional content"); err != nil {
		logging.LogWarning("failed to commit test file changes: %v", err)
	}

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
	testDir := filepath.Join(contentStorage.GetDocsPath(), "test")

	err := filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			testFiles = append(testFiles, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	statuses := []files.Status{files.StatusDraft, files.StatusPublished, files.StatusPublished, files.StatusDraft}
	priorities := []files.Priority{files.PriorityLow, files.PriorityMedium, files.PriorityHigh}
	fileTypes := []files.Filetype{files.FileTypeFleeting, files.FileTypeLiterature, files.FileTypePermanent, files.FileTypeJournaling}

	for i, file := range testFiles {
		filename := filepath.Base(file)
		relPath := pathutils.ToRelative(file)
		// ensure relPath doesn't already have docs/ prefix to avoid double prefix
		relPath = strings.TrimPrefix(relPath, "docs/")
		// add docs/ prefix for metadata path
		metadataPath := filepath.Join("docs", relPath)
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
		fileType := fileTypes[i%len(fileTypes)]

		tags := extractFilenameTags(filename)

		// Generate PARA data based on filename patterns and position
		paraProjects := generatePARAProjects(filename, i)
		paraAreas := generatePARAareas(filename, collection, i)
		paraResources := generatePARAResources(filename, tags, i)
		paraArchive := generatePARAArchive(filename, i)

		var parents []string
		if i > 0 {
			parentCount := (i % 3)
			if parentCount > 2 {
				parentCount = 2
			}

			for j := 0; j < parentCount && j < i; j++ {
				parentIdx := i - 1 - (j * 2)
				if parentIdx >= 0 && parentIdx < i {
					parentPath := pathutils.ToRelative(testFiles[parentIdx])
					parentMetadataPath := filepath.Join("docs", parentPath)
					if parentMetadataPath != metadataPath && !contains(parents, parentMetadataPath) {
						parents = append(parents, parentMetadataPath)
					}
				}
			}
		}

		metadata := &files.Metadata{
			Name:       filename,
			Path:       metadataPath,
			CreatedAt:  time.Date(2025, 9, createDay, 8+(i%8), (i*7)%60, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, editDay, 10+(i%6), (i*13)%60, 0, 0, time.UTC),
			Collection: collection,
			Folders:    validFolders,
			Tags:       tags,
			Boards:     []string{},
			Parents:    parents,
			FileType:   fileType,
			Status:     status,
			Priority:   priority,
			PARA: files.PARA{
				Projects:  paraProjects,
				Areas:     paraAreas,
				Resources: paraResources,
				Archive:   paraArchive,
			},
		}

		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError("failed to save test metadata for %s: %v", file, err)
		}
	}

	return nil
}

func generatePARAProjects(filename string, index int) []string {
	var projects []string

	// Pattern-based assignment
	if strings.Contains(filename, "testA") {
		projects = append(projects, "knowledge_system")
		if index%3 == 0 {
			projects = append(projects, "documentation_update")
		}
	}
	if strings.Contains(filename, "testB") {
		projects = append(projects, "search_improvement")
		if index%4 == 0 {
			projects = append(projects, "user_interface")
		}
	}
	if strings.Contains(filename, "testC") {
		projects = append(projects, "performance_optimization")
	}

	return projects
}

func generatePARAareas(filename, collection string, index int) []string {
	var areas []string

	// Collection-based areas
	if collection == "test" {
		areas = append(areas, "testing")
	}

	// Pattern-based areas
	if strings.Contains(filename, "AA") {
		areas = append(areas, "documentation")
	}
	if strings.Contains(filename, "AB") {
		areas = append(areas, "development")
	}
	if strings.Contains(filename, "AC") {
		areas = append(areas, "research")
	}

	// Index-based assignment for variety
	switch index % 5 {
	case 0:
		areas = append(areas, "knowledge_management")
	case 1:
		areas = append(areas, "process_improvement")
	case 2:
		areas = append(areas, "team_coordination")
	}

	return areas
}

func generatePARAResources(filename string, tags []string, index int) []string {
	var resources []string

	// Tag-based resources
	for _, tag := range tags {
		switch tag {
		case "testA":
			resources = append(resources, "methodology_references")
		case "testB":
			resources = append(resources, "technical_specs")
		case "testC":
			resources = append(resources, "best_practices")
		}
	}

	// Pattern-based resources
	if strings.Contains(filename, "B") && index%3 == 0 {
		resources = append(resources, "templates_and_examples")
	}

	return resources
}

func generatePARAArchive(filename string, index int) []string {
	var archive []string

	// Occasionally add to archive (simulate old projects)
	if index%7 == 0 {
		archive = append(archive, "old_system_migration")
	}
	if index%11 == 0 {
		archive = append(archive, "deprecated_processes")
	}

	return archive
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
