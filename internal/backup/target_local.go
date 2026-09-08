package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"knov/internal/utils"
)

const (
	archiveExt   = ".tar.gz"
	lockExt      = ".locked"
	eventLogFile = "log.json"
)

// localTarget stores each backup set as a single <name>.tar.gz file under root.
type localTarget struct {
	root string
}

// NewLocalTarget creates (if needed) and returns a filesystem-backed BackupTarget rooted at root.
func NewLocalTarget(root string) (BackupTarget, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &localTarget{root: root}, nil
}

func (t *localTarget) path(name string) string {
	return filepath.Join(t.root, name+archiveExt)
}

func (t *localTarget) lockPath(name string) string {
	return filepath.Join(t.root, name+lockExt)
}

func (t *localTarget) logPath() string {
	return filepath.Join(t.root, eventLogFile)
}

func (t *localTarget) Write(name string, r io.Reader) error {
	if _, err := os.Stat(t.path(name)); err == nil {
		return fmt.Errorf("backup set %s already exists", name)
	}

	tmp, err := os.CreateTemp(t.root, "*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, t.path(name)); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func (t *localTarget) List() ([]string, error) {
	entries, err := os.ReadDir(t.root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), archiveExt) {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), archiveExt))
	}
	sort.Strings(names)
	return names, nil
}

func (t *localTarget) Read(name string) (io.ReadCloser, error) {
	return os.Open(t.path(name))
}

func (t *localTarget) Delete(name string) error {
	if err := os.Remove(t.path(name)); err != nil {
		return err
	}
	os.Remove(t.lockPath(name)) // best-effort, the archive is already gone either way
	return nil
}

// Lock creates an empty marker file next to name's archive - a separate file rather than
// metadata inside the archive, since Rotate needs to check it without reading the archive.
func (t *localTarget) Lock(name string) error {
	f, err := os.Create(t.lockPath(name))
	if err != nil {
		return err
	}
	return f.Close()
}

func (t *localTarget) Unlock(name string) error {
	err := os.Remove(t.lockPath(name))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func (t *localTarget) Locked(name string) (bool, error) {
	_, err := os.Stat(t.lockPath(name))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (t *localTarget) LockedNames() (map[string]bool, error) {
	entries, err := os.ReadDir(t.root)
	if err != nil {
		return nil, err
	}
	locked := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), lockExt) {
			locked[strings.TrimSuffix(e.Name(), lockExt)] = true
		}
	}
	return locked, nil
}

// LogEvent appends a new event to the target's history file (a single JSON array, rewritten
// atomically) - callers (Run/Restore) are already serialized by job.backupMu, so a plain
// read-modify-write needs no extra locking of its own.
func (t *localTarget) LogEvent(kind EventKind, set string, source EventSource) error {
	events, err := t.Events()
	if err != nil {
		return err
	}
	events = append(events, Event{Kind: kind, Set: set, Time: time.Now(), Source: source})
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	return utils.WriteFileAtomic(t.logPath(), data, 0644)
}

func (t *localTarget) Events() ([]Event, error) {
	data, err := os.ReadFile(t.logPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}
