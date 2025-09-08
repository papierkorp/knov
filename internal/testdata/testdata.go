// Package testdata - Test data setup and management
package testdata

import (
	"os"
	"os/exec"
	"path/filepath"

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
	if err := os.RemoveAll("data"); err != nil {
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

	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}

	cmd := exec.Command("cp", "-r", "internal/testdata/testfiles/.", "data/")
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to copy test files: %v", err)
		return err
	}

	return nil
}

func createGitOperations() error {
	logging.LogInfo("creating git operations")

	dataDir := "data"

	// Initialize git if needed
	gitDir := filepath.Join(dataDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = dataDir
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// Initial commit
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dataDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial test data", "--allow-empty")
	cmd.Dir = dataDir
	cmd.Run()

	// Create test file
	testFile := filepath.Join(dataDir, "test_created_file.md")
	os.WriteFile(testFile, []byte("# Test File Created by API"), 0644)

	cmd = exec.Command("git", "add", "test_created_file.md")
	cmd.Dir = dataDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add dynamically created test file")
	cmd.Dir = dataDir
	cmd.Run()

	// Move ai.md to projects
	os.MkdirAll(filepath.Join(dataDir, "projects"), 0755)
	os.Rename(filepath.Join(dataDir, "ai.md"), filepath.Join(dataDir, "projects", "ai.md"))

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dataDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "move ai.md to projects folder", "--allow-empty")
	cmd.Dir = dataDir
	cmd.Run()

	// Create project notes
	projectNotes := filepath.Join(dataDir, "projects", "project_notes.md")
	os.WriteFile(projectNotes, []byte("# Another Test File"), 0644)

	cmd = exec.Command("git", "add", "projects/project_notes.md")
	cmd.Dir = dataDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add project notes")
	cmd.Dir = dataDir
	cmd.Run()

	// Remove test file
	os.Remove(testFile)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dataDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "remove test file", "--allow-empty")
	cmd.Dir = dataDir
	cmd.Run()

	// Create test files with links
	child1 := filepath.Join(dataDir, "child1.md")
	os.WriteFile(child1, []byte("# Child Document\n\nThis links to [[projects/ai.md]]"), 0644)

	child2 := filepath.Join(dataDir, "child2.md")
	os.WriteFile(child2, []byte("# Child Document 2\n\nThis also links to [[projects/ai.md]]"), 0644)

	grandchild := filepath.Join(dataDir, "grandchild.md")
	os.WriteFile(grandchild, []byte("# Grandchild\n\nParent: [[child1.md]]"), 0644)

	cmd = exec.Command("git", "add", "*.md")
	cmd.Dir = dataDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add test files with parent relationships")
	cmd.Dir = dataDir
	cmd.Run()

	return nil
}

func setupTestMetadata() error {
	logging.LogInfo("creating test metadata")

	if err := os.MkdirAll("config/.metadata", 0755); err != nil {
		return err
	}

	metadata := `{
  "data/child1.md": {
    "name": "child1.md",
    "path": "data/child1.md",
    "createdAt": "2025-09-08T21:00:00Z",
    "lastEdited": "2025-09-08T21:00:00Z",
    "project": "test",
    "folders": [],
    "tags": [],
    "boards": ["default"],
    "ancestor": [],
    "parents": ["data/projects/ai.md"],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "medium",
    "size": 100
  },
  "data/child2.md": {
    "name": "child2.md",
    "path": "data/child2.md",
    "createdAt": "2025-09-08T21:00:00Z",
    "lastEdited": "2025-09-08T21:00:00Z",
    "project": "test",
    "folders": [],
    "tags": [],
    "boards": ["default"],
    "ancestor": [],
    "parents": ["data/projects/ai.md"],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "medium",
    "size": 100
  },
  "data/grandchild.md": {
    "name": "grandchild.md",
    "path": "data/grandchild.md",
    "createdAt": "2025-09-08T21:00:00Z",
    "lastEdited": "2025-09-08T21:00:00Z",
    "project": "test",
    "folders": [],
    "tags": [],
    "boards": ["default"],
    "ancestor": [],
    "parents": ["data/child1.md"],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "medium",
    "size": 100
  }
}`

	return os.WriteFile("config/.metadata/metadata.json", []byte(metadata), 0644)
}
