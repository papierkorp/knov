// Package search provides different search engine implementations
package search

import (
	"os/exec"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
)

// GrepEngine ..
type GrepEngine struct {
	dataDir string
}

// Initialize ..
func (g *GrepEngine) Initialize() error {
	g.dataDir = configmanager.GetAppConfig().DataPath
	return nil
}

// IndexAllFiles ..
func (g *GrepEngine) IndexAllFiles() error {
	return nil
}

// SearchFiles ..
func (g *GrepEngine) SearchFiles(query string, limit int) ([]files.File, error) {
	if limit <= 0 {
		limit = 20
	}

	cmd := exec.Command("grep", "-r", "-l", "-i", "--include=*.md", query, g.dataDir)
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return []files.File{}, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var results []files.File

	for i, line := range lines {
		if i >= limit || line == "" {
			break
		}

		relativePath, err := filepath.Rel(g.dataDir, line)
		if err != nil {
			relativePath = line
		}

		results = append(results, files.File{
			Name: filepath.Base(relativePath),
			Path: relativePath,
		})
	}
	return results, nil
}
