// Package metadataStorage - YAML front matter backend implementation.
// Metadata is stored as YAML front matter (--- block) directly at the top of
// every docs file. No separate storage/metadata folder is used.
// Media files are silently skipped as they cannot carry text front matter.
// The parser package is responsible for stripping the block before rendering.
package metadataStorage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/configmanager"
	"knov/internal/logging"

	"gopkg.in/yaml.v3"
)

var frontMatterDelimiter = []byte("---\n")
var frontMatterClose = []byte("\n---\n")

// yamlFrontmatterStorage implements MetadataStorage by embedding YAML front matter
// directly inside docs files. No separate storage folder is created or used.
type yamlFrontmatterStorage struct {
	docsPath string
	mutex    sync.RWMutex
}

// newYAMLStorage creates a new YAML front matter storage instance.
// storagePath is intentionally ignored - data lives in the docs folder.
func newYAMLStorage(_ string) (*yamlFrontmatterStorage, error) {
	docsPath := filepath.Join(configmanager.GetAppConfig().DataPath, "docs")
	if err := os.MkdirAll(docsPath, 0755); err != nil {
		return nil, err
	}
	logging.LogDebug(logging.KeyApp, "yaml front matter storage using docs path: %s", docsPath)
	return &yamlFrontmatterStorage{docsPath: docsPath}, nil
}

// docFilePath returns the full filesystem path for a metadata key.
// Returns empty string for media/ keys (not supported).
func (ys *yamlFrontmatterStorage) docFilePath(key string) string {
	if !strings.HasPrefix(key, "docs/") {
		return "" // media and other prefixes are not supported
	}
	rel := strings.TrimPrefix(key, "docs/")
	return filepath.Join(ys.docsPath, filepath.FromSlash(rel))
}

// stripFrontMatter splits content into (frontmatterYAML bytes, body bytes).
// Returns (nil, content) when no front matter is present.
func stripFrontMatter(content []byte) (frontmatter []byte, body []byte) {
	if !bytes.HasPrefix(content, frontMatterDelimiter) {
		return nil, content
	}
	rest := content[len(frontMatterDelimiter):]
	idx := bytes.Index(rest, frontMatterClose)
	if idx < 0 {
		return nil, content // malformed — treat as no front matter
	}
	return rest[:idx], rest[idx+len(frontMatterClose):]
}

// Get reads the file and returns its front matter as JSON.
// Returns nil (not an error) for media files or files without front matter.
func (ys *yamlFrontmatterStorage) Get(key string) ([]byte, error) {
	ys.mutex.RLock()
	defer ys.mutex.RUnlock()

	filePath := ys.docFilePath(key)
	if filePath == "" {
		return nil, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		logging.LogError(logging.KeyApp, "yaml front matter: failed to read %s: %v", filePath, err)
		return nil, err
	}

	frontmatter, _ := stripFrontMatter(content)
	if frontmatter == nil {
		return nil, nil
	}

	var raw interface{}
	if err := yaml.Unmarshal(frontmatter, &raw); err != nil {
		logging.LogError(logging.KeyApp, "yaml front matter: failed to parse front matter in %s: %v", filePath, err)
		return nil, err
	}

	jsonData, err := json.Marshal(raw)
	if err != nil {
		logging.LogError(logging.KeyApp, "yaml front matter: failed to marshal json for %s: %v", key, err)
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "yaml front matter: retrieved metadata for key: %s", key)
	return jsonData, nil
}

// Set writes metadata as YAML front matter at the top of the docs file.
// If the file does not yet exist the operation is a no-op (file must exist).
// Media keys are silently ignored.
func (ys *yamlFrontmatterStorage) Set(key string, data []byte) error {
	ys.mutex.Lock()
	defer ys.mutex.Unlock()

	filePath := ys.docFilePath(key)
	if filePath == "" {
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.LogDebug(logging.KeyApp, "yaml front matter: file does not exist yet, skipping set for %s", key)
			return nil
		}
		logging.LogError(logging.KeyApp, "yaml front matter: failed to read %s: %v", filePath, err)
		return err
	}

	// convert incoming JSON → map → YAML
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		logging.LogError(logging.KeyApp, "yaml front matter: data for %s is not valid json: %v", key, err)
		return err
	}

	yamlBytes, err := yaml.Marshal(raw)
	if err != nil {
		logging.LogError(logging.KeyApp, "yaml front matter: failed to marshal yaml for %s: %v", key, err)
		return err
	}

	// strip any existing front matter, prepend fresh one
	_, body := stripFrontMatter(content)

	var out bytes.Buffer
	fmt.Fprintf(&out, "---\n%s---\n", yamlBytes)
	out.Write(body)

	if err := os.WriteFile(filePath, out.Bytes(), 0644); err != nil {
		logging.LogError(logging.KeyApp, "yaml front matter: failed to write %s: %v", filePath, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "yaml front matter: stored metadata for key: %s", key)
	return nil
}

// Delete strips the YAML front matter from the file, leaving only the body.
func (ys *yamlFrontmatterStorage) Delete(key string) error {
	ys.mutex.Lock()
	defer ys.mutex.Unlock()

	filePath := ys.docFilePath(key)
	if filePath == "" {
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		logging.LogError(logging.KeyApp, "yaml front matter: failed to read %s: %v", filePath, err)
		return err
	}

	_, body := stripFrontMatter(content)
	if err := os.WriteFile(filePath, body, 0644); err != nil {
		logging.LogError(logging.KeyApp, "yaml front matter: failed to write %s after delete: %v", filePath, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "yaml front matter: deleted metadata for key: %s", key)
	return nil
}

// GetAll walks all docs files and returns front matter entries as JSON.
func (ys *yamlFrontmatterStorage) GetAll() (map[string][]byte, error) {
	ys.mutex.RLock()
	defer ys.mutex.RUnlock()

	result := make(map[string][]byte)

	err := filepath.Walk(ys.docsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "yaml front matter: cannot read %s: %v", path, err)
			return nil
		}

		frontmatter, _ := stripFrontMatter(content)
		if frontmatter == nil {
			return nil
		}

		var raw interface{}
		if err := yaml.Unmarshal(frontmatter, &raw); err != nil {
			logging.LogWarning(logging.KeyApp, "yaml front matter: cannot parse front matter in %s: %v", path, err)
			return nil
		}

		jsonData, err := json.Marshal(raw)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "yaml front matter: cannot marshal json for %s: %v", path, err)
			return nil
		}

		relPath, err := filepath.Rel(ys.docsPath, path)
		if err != nil {
			return nil
		}
		key := "docs/" + filepath.ToSlash(relPath)
		result[key] = jsonData
		return nil
	})

	if err != nil {
		logging.LogError(logging.KeyApp, "yaml front matter: failed to walk docs path: %v", err)
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "yaml front matter: retrieved %d metadata entries", len(result))
	return result, nil
}

// Exists returns true if the file exists and has YAML front matter.
func (ys *yamlFrontmatterStorage) Exists(key string) bool {
	ys.mutex.RLock()
	defer ys.mutex.RUnlock()

	filePath := ys.docFilePath(key)
	if filePath == "" {
		return false
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	fm, _ := stripFrontMatter(content)
	return fm != nil
}

// GetBackendType returns the backend type identifier.
func (ys *yamlFrontmatterStorage) GetBackendType() string {
	return "yaml"
}

// Cleanup strips front matter from all docs files in one pass
func (ys *yamlFrontmatterStorage) Cleanup() error {
	ys.mutex.Lock()
	defer ys.mutex.Unlock()

	var cleaned, failed int

	err := filepath.Walk(ys.docsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "yaml cleanup: cannot read %s: %v", path, err)
			failed++
			return nil
		}

		_, body := stripFrontMatter(content)
		if len(body) == len(content) {
			return nil // no front matter — nothing to do
		}

		if err := os.WriteFile(path, body, 0644); err != nil {
			logging.LogError(logging.KeyApp, "yaml cleanup: failed to write %s: %v", path, err)
			failed++
			return nil
		}

		cleaned++
		return nil
	})

	if err != nil {
		logging.LogError(logging.KeyApp, "yaml metadata cleanup: walk error: %v", err)
		return err
	}

	logging.LogInfo(logging.KeyApp, "yaml metadata cleanup: stripped front matter from %d files (%d failed)", cleaned, failed)
	return nil
}
