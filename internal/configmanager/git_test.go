package configmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitGitRepository(t *testing.T) {
	// Save original config
	originalConfig := configManager

	tests := []struct {
		name           string
		config         ConfigManager
		existingRepo   bool
		expectedAction string
	}{
		{
			name: "no repo, no config - should init",
			config: ConfigManager{
				Git: ConfigGit{
					InitRepo:      false,
					RepositoryURL: "",
					DataPath:      "test_data_1",
				},
			},
			existingRepo:   false,
			expectedAction: "init",
		},
		{
			name: "no repo, initRepo true - should init",
			config: ConfigManager{
				Git: ConfigGit{
					InitRepo:      true,
					RepositoryURL: "",
					DataPath:      "test_data_2",
				},
			},
			existingRepo:   false,
			expectedAction: "init",
		},
		{
			name: "no repo, with repositoryURL - should clone",
			config: ConfigManager{
				Git: ConfigGit{
					InitRepo:      false,
					RepositoryURL: "https://github.com/test/repo.git",
					DataPath:      "test_data_3",
				},
			},
			existingRepo:   false,
			expectedAction: "clone",
		},
		{
			name: "existing repo, no config - should skip",
			config: ConfigManager{
				Git: ConfigGit{
					InitRepo:      false,
					RepositoryURL: "",
					DataPath:      "test_data_4",
				},
			},
			existingRepo:   true,
			expectedAction: "skip",
		},
		{
			name: "existing repo, initRepo true - should skip",
			config: ConfigManager{
				Git: ConfigGit{
					InitRepo:      true,
					RepositoryURL: "",
					DataPath:      "test_data_5",
				},
			},
			existingRepo:   true,
			expectedAction: "skip",
		},
		{
			name: "existing repo, with repositoryURL - should skip",
			config: ConfigManager{
				Git: ConfigGit{
					InitRepo:      false,
					RepositoryURL: "https://github.com/test/repo.git",
					DataPath:      "test_data_6",
				},
			},
			existingRepo:   true,
			expectedAction: "skip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test directory
			testDir := tt.config.Git.DataPath
			defer os.RemoveAll(testDir)

			// Create existing repo if needed
			if tt.existingRepo {
				os.MkdirAll(testDir, 0755)
				gitDir := filepath.Join(testDir, ".git")
				os.MkdirAll(gitDir, 0755)
			}

			// Set test config
			configManager = tt.config

			// Run the function
			err := initGitRepository()

			// Verify results based on expected action
			switch tt.expectedAction {
			case "init":
				if err != nil {
					t.Errorf("expected successful init, got error: %v", err)
				}
				gitDir := filepath.Join(testDir, ".git")
				if _, err := os.Stat(gitDir); os.IsNotExist(err) {
					t.Errorf("expected .git directory to be created")
				}
				// Check if initRepo was set to false
				if configManager.Git.InitRepo != false {
					t.Errorf("expected initRepo to be set to false after init")
				}

			case "clone":
				// Clone will fail in test (no real repo), but we can check the attempt
				if err == nil {
					t.Errorf("expected clone error in test environment")
				}
				if err != nil && err.Error() != "failed to clone repository: exit status 128" {
					// This is expected since we're using a fake URL
					t.Logf("expected clone error: %v", err)
				}

			case "skip":
				if err != nil {
					t.Errorf("expected no error when skipping, got: %v", err)
				}
				// Repo should still exist if it existed before
				if tt.existingRepo {
					gitDir := filepath.Join(testDir, ".git")
					if _, err := os.Stat(gitDir); os.IsNotExist(err) {
						t.Errorf("existing repo should not be removed")
					}
				}
			}
		})
	}

	// Restore original config
	configManager = originalConfig
}

func TestInitGitRepositoryWithEmptyDataPath(t *testing.T) {
	// Save original config
	originalConfig := configManager

	// Test with empty DataPath (should default to "data")
	configManager = ConfigManager{
		Git: ConfigGit{
			InitRepo:      true,
			RepositoryURL: "",
			DataPath:      "", // Empty path
		},
	}

	testDir := "data"
	defer os.RemoveAll(testDir)

	err := initGitRepository()
	if err != nil {
		t.Errorf("expected successful init with default path, got error: %v", err)
	}

	gitDir := filepath.Join("data", ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf("expected .git directory to be created in default 'data' path")
	}

	// Restore original config
	configManager = originalConfig
}
