// Package testdata - Test data setup and management
package testdata

import (
	"os"
	"os/exec"
	"path/filepath"

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
	if err := os.RemoveAll(configmanager.DataPath); err != nil {
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

	if err := os.MkdirAll(configmanager.DataPath, 0755); err != nil {
		return err
	}

	cmd := exec.Command("cp", "-r", "internal/testdata/testfiles/.", configmanager.DataPath+"/")
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to copy test files: %v", err)
		return err
	}

	return nil
}

func createGitOperations() error {
	logging.LogInfo("creating git operations")

	dataDir := configmanager.DataPath

	// Initialize git if needed
	gitDir := filepath.Join(dataDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = dataDir
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// Copy test files from testfiles directory
	cmd := exec.Command("cp", "-r", "internal/testdata/testfiles/.", dataDir+"/")
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to copy test files: %v", err)
		return err
	}

	// Initial commit
	cmd = exec.Command("git", "add", ".")
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

	return nil
}

func setupTestMetadata() error {
	logging.LogInfo("creating test metadata")

	if err := os.MkdirAll("config/.metadata", 0755); err != nil {
		return err
	}

	metadata := `{
  "data/getting-started.md": {
    "name": "getting-started.md",
    "path": "data/getting-started.md",
    "createdAt": "2025-09-08T21:00:00Z",
    "lastEdited": "2025-09-08T21:00:00Z",
    "project": "documentation",
    "folders": [],
    "tags": ["guide", "onboarding", "getting-started"],
    "boards": ["default"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "high",
    "size": 0
  },
  "data/project-overview.md": {
    "name": "project-overview.md", 
    "path": "data/project-overview.md",
    "createdAt": "2025-09-08T20:00:00Z",
    "lastEdited": "2025-09-08T22:00:00Z",
    "project": "management",
    "folders": [],
    "tags": ["project", "overview", "status"],
    "boards": ["default", "management"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "high",
    "size": 0
  },
  "data/technical-documentation.md": {
    "name": "technical-documentation.md",
    "path": "data/technical-documentation.md",
    "createdAt": "2025-09-08T19:00:00Z",
    "lastEdited": "2025-09-08T23:00:00Z",
    "project": "technical",
    "folders": [],
    "tags": ["technical", "api", "documentation"],
    "boards": ["default", "technical"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "medium",
    "size": 0
  },
  "data/meeting-notes.md": {
    "name": "meeting-notes.md",
    "path": "data/meeting-notes.md",
    "createdAt": "2025-09-11T10:00:00Z",
    "lastEdited": "2025-09-11T15:00:00Z",
    "project": "management",
    "folders": [],
    "tags": ["meeting", "sprint", "planning"],
    "boards": ["default", "meetings"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "medium",
    "size": 0
  },
  "data/troubleshooting.md": {
    "name": "troubleshooting.md",
    "path": "data/troubleshooting.md",
    "createdAt": "2025-09-07T14:00:00Z",
    "lastEdited": "2025-09-10T16:00:00Z",
    "project": "support",
    "folders": [],
    "tags": ["troubleshooting", "help", "debug"],
    "boards": ["default", "support"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "high",
    "size": 0
  },
  "data/projects/backend-api.md": {
    "name": "backend-api.md",
    "path": "data/projects/backend-api.md",
    "createdAt": "2025-09-05T09:00:00Z",
    "lastEdited": "2025-09-11T14:00:00Z",
    "project": "backend",
    "folders": ["projects"],
    "tags": ["backend", "api", "development", "in-progress"],
    "boards": ["default", "development"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "todo",
    "status": "draft",
    "priority": "high",
    "size": 0
  },
  "data/projects/frontend-redesign.md": {
    "name": "frontend-redesign.md",
    "path": "data/projects/frontend-redesign.md",
    "createdAt": "2025-09-06T11:00:00Z",
    "lastEdited": "2025-09-09T17:00:00Z",
    "project": "frontend",
    "folders": ["projects"],
    "tags": ["frontend", "ui", "redesign", "planning"],
    "boards": ["default", "design"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "todo",
    "status": "draft",
    "priority": "medium",
    "size": 0
  },
  "data/projects/database-migration.md": {
    "name": "database-migration.md",
    "path": "data/projects/database-migration.md",
    "createdAt": "2025-08-15T08:00:00Z",
    "lastEdited": "2025-09-01T12:00:00Z",
    "project": "infrastructure",
    "folders": ["projects"],
    "tags": ["database", "migration", "completed", "infrastructure"],
    "boards": ["default", "infrastructure"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "high",
    "size": 0
  },
  "data/guides/user-manual.md": {
    "name": "user-manual.md",
    "path": "data/guides/user-manual.md",
    "createdAt": "2025-09-04T13:00:00Z",
    "lastEdited": "2025-09-08T18:00:00Z",
    "project": "documentation",
    "folders": ["guides"],
    "tags": ["user", "manual", "guide", "help"],
    "boards": ["default", "documentation"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "medium",
    "size": 0
  },
  "data/guides/developer-setup.md": {
    "name": "developer-setup.md",
    "path": "data/guides/developer-setup.md",
    "createdAt": "2025-09-03T16:00:00Z",
    "lastEdited": "2025-09-07T10:00:00Z",
    "project": "technical",
    "folders": ["guides"],
    "tags": ["developer", "setup", "guide", "technical"],
    "boards": ["default", "technical"],
    "ancestor": [],
    "parents": [],
    "kids": [],
    "usedLinks": [],
    "linksToHere": [],
    "type": "note",
    "status": "published",
    "priority": "medium",
    "size": 0
  }
}`

	if err := os.WriteFile("config/.metadata/metadata.json", []byte(metadata), 0644); err != nil {
		return err
	}

	// Let the app automatically detect and create links
	return files.MetaDataLinksRebuild()
}
