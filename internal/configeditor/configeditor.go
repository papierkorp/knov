// Package configeditor holds the shared persistence + paired-file ceremony for
// editors whose logic lives in configStorage and which present a generated
// markdown file (filter editor, ...). Each editor keeps its own config schema,
// form parsing and rendering; only the store/write/tag/delete boilerplate lives
// here.
package configeditor

import (
	"fmt"
	"path/filepath"
	"strings"

	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// Kind describes one config-backed editor type. Build it with MustNew so the
// invariants (prefix ends in "/", extKey set) always hold.
type Kind struct {
	prefix string           // configStorage key prefix, always ends in "/"
	editor files.EditorType // editor tag written to the paired file's metadata
	extKey string           // configmanager.ExtensionForEditor key, e.g. "index"
}

// MustNew returns a Kind for an editor whose config is stored under prefix and
// whose paired file uses the extKey extension. It panics on an empty extKey since
// every Kind is a package-level singleton built at startup.
func MustNew(prefix string, editor files.EditorType, extKey string) Kind {
	if extKey == "" {
		panic("configeditor.MustNew: extKey must not be empty")
	}
	return Kind{
		prefix: strings.TrimSuffix(prefix, "/") + "/",
		editor: editor,
		extKey: extKey,
	}
}

// key returns the configStorage key for id, rejecting ids (e.g. "../../x" from a
// hand-edited .book or a crafted request) that would resolve outside the prefix.
func (k Kind) key(id string) (string, error) {
	root := filepath.Clean(k.label())
	if p := filepath.Join(root, id); p == root || !pathutils.PathContains(root, p) {
		return "", fmt.Errorf("invalid %s id: %q", k.label(), id)
	}
	return k.prefix + id, nil
}

// label is the prefix without its trailing slash, for log/error messages.
func (k Kind) label() string { return strings.TrimSuffix(k.prefix, "/") }

// PairedPath returns the docs-relative path of the paired file for an id,
// e.g. "my/thing.index" or "my/thing.md" depending on the useExtensionIndex setting.
func (k Kind) PairedPath(id string) string {
	return id + configmanager.ExtensionForEditor(k.extKey)
}

// IDFromPath is the inverse of PairedPath: it strips the configured extension
// from a docs-relative path to recover the id.
func (k Kind) IDFromPath(relPath string) string {
	return strings.TrimSuffix(relPath, configmanager.ExtensionForEditor(k.extKey))
}

// Get loads the raw stored config bytes for id, or nil when absent.
func (k Kind) Get(id string) ([]byte, error) {
	key, err := k.key(id)
	if err != nil {
		return nil, err
	}
	return configStorage.Get(key)
}

// Set stores the raw config bytes for id.
func (k Kind) Set(id string, data []byte) error {
	key, err := k.key(id)
	if err != nil {
		return err
	}
	return configStorage.Set(key, data)
}

// List returns all stored ids for this kind.
func (k Kind) List() ([]string, error) {
	keys, err := configStorage.List(k.prefix)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(keys))
	for i, key := range keys {
		ids[i] = strings.TrimPrefix(key, k.prefix)
	}
	return ids, nil
}

// WritePaired writes markdown as the paired physical file for id and tags its
// metadata so the matching editor opens it. The file is always overwritten so it
// stays in sync with the stored config.
func (k Kind) WritePaired(id string, markdown []byte) error {
	pairedPath := k.PairedPath(id)
	fullPath := pathutils.ToDocsPath(pairedPath)

	if err := contentStorage.WriteFile(fullPath, markdown, 0644); err != nil {
		return fmt.Errorf("failed to write %s paired file %s: %w", k.label(), pairedPath, err)
	}

	// the physical file may use a non-markdown extension (e.g. ".index"), so the
	// editor type must be forced rather than left to extension inference
	normalized := pathutils.ToWithPrefix(pairedPath)
	if err := files.MetaDataSync(normalized); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to save metadata for %s paired file %s: %v", k.label(), pairedPath, err)
	} else if err := files.SetEditor(normalized, k.editor); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set editor for %s paired file %s: %v", k.label(), pairedPath, err)
	}
	return nil
}

// Delete removes the stored config for id and its paired physical file + metadata.
func (k Kind) Delete(id string) error {
	key, err := k.key(id)
	if err != nil {
		return err
	}
	pairedPath := k.PairedPath(id)
	fullPath := pathutils.ToDocsPath(pairedPath)
	if err := contentStorage.DeleteFile(fullPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to delete %s paired file %s: %v", k.label(), fullPath, err)
	}
	if err := files.MetaDataDelete(pathutils.ToWithPrefix(pairedPath)); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to delete %s paired file metadata %s: %v", k.label(), pairedPath, err)
	}
	return configStorage.Delete(key)
}
