// Package backuptest - probe seeding/mutation/verification helpers, scratch backup targets and
// archive extraction, plus the sqlite-storage reinit needed after a direct backup.Restore call.
package backuptest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/backup"
	"knov/internal/cacheStorage"
	"knov/internal/chatStorage"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/kanbanStorage"
	"knov/internal/metadataStorage"
	"knov/internal/notificationStorage"
	"knov/internal/searchStorage"
	"knov/internal/test"
)

// setNameLayout mirrors backup package's own unexported nameLayout - needed here to fabricate
// synthetic, past-dated set names for the rotate/auto-backup/log cases, since backup exposes no
// name-formatting helper of its own (only ParseSetTime, the reverse direction).
const setNameLayout = "2006-01-02T15-04-05"

func setNameAt(t time.Time) string {
	return t.Format(setNameLayout)
}

const (
	probeMetaKey        = "backuptest-probe-meta"
	probeCacheKey       = "backuptest-probe-cache"
	probeConfigKey      = "backuptest-probe-config"
	probeSearchPath     = "backuptest/probe-search.md"
	probeChatFilePath   = "backuptest/probe-chat.md"
	probeKanbanFilePath = "backuptest/probe-kanban.md"
	probeKanbanFolder   = "backuptest"
	probeNotifLevel     = "info"
)

// probeMetaTitle/probeMetaMutatedTitle are the metadata probe's only interesting values - unlike
// cache/config, metadataStorage.Set's real (sqlite-enforced) contract is a JSON-encoded Metadata
// object, not an arbitrary blob, and sqlite's Get rebuilds JSON from named columns rather than
// echoing back whatever bytes Set was given - so the probe must use a recognized field ("title")
// and verifyProbes must compare that field, not the raw bytes, for this to work on every backend.
const (
	probeMetaTitle        = "backuptest-meta-value"
	probeMetaMutatedTitle = "backuptest-meta-mutated"
)

var (
	probeMetaValue        = []byte(fmt.Sprintf(`{"title":%q}`, probeMetaTitle))
	probeMetaMutatedValue = []byte(fmt.Sprintf(`{"title":%q}`, probeMetaMutatedTitle))
	probeCacheValue       = []byte("backuptest-cache-value")
	probeConfigValue      = []byte("backuptest-config-value")
	probeSearchValue      = []byte("backuptest-search-value")
	probeChatContent      = "backuptest-chat-value"
	probeNotifMessage     = "backuptest-notif-value"
)

// probes holds the ids assigned to the dynamically-created probe rows (chat message,
// notification) so a later mutate/verify step can address the exact same row.
type probes struct {
	chatID  string
	notifID string
}

// seedProbes writes one recognizable, test-only entry into every registered storage - a
// generic key for the plain key/value backends (metadata/cache/config), a real row for the
// row-oriented ones (chat/notifications), an indexed file for search, and an event for kanban
// (append-only, no single-row identity to mutate in place).
func seedProbes() (*probes, error) {
	if err := metadataStorage.Set(probeMetaKey, probeMetaValue); err != nil {
		return nil, err
	}
	if err := cacheStorage.Set(probeCacheKey, probeCacheValue); err != nil {
		return nil, err
	}
	if err := configStorage.Set(probeConfigKey, probeConfigValue); err != nil {
		return nil, err
	}
	if err := searchStorage.IndexFile(probeSearchPath, probeSearchValue); err != nil {
		return nil, err
	}
	if err := kanbanStorage.LogEvent(probeKanbanFilePath, probeKanbanFolder, "", "inbox"); err != nil {
		return nil, err
	}
	msg, err := chatStorage.Add(probeChatContent, probeChatFilePath)
	if err != nil {
		return nil, err
	}
	n, err := notificationStorage.Add(probeNotifLevel, probeNotifMessage, false)
	if err != nil {
		return nil, err
	}
	return &probes{chatID: msg.ID, notifID: n.ID}, nil
}

// mutateProbes changes or deletes every probe seeded above, so a later restore has something
// real to reverse. Kanban is append-only, so its "mutation" is a second event a restore back to
// the pre-mutation snapshot should make disappear again.
func mutateProbes(p *probes) error {
	if err := metadataStorage.Set(probeMetaKey, probeMetaMutatedValue); err != nil {
		return err
	}
	if err := cacheStorage.Delete(probeCacheKey); err != nil {
		return err
	}
	if err := configStorage.Set(probeConfigKey, []byte("mutated")); err != nil {
		return err
	}
	if err := searchStorage.DeleteIndexedContent(probeSearchPath); err != nil {
		return err
	}
	if err := kanbanStorage.LogEvent(probeKanbanFilePath, probeKanbanFolder, "inbox", "inprogress"); err != nil {
		return err
	}
	if err := chatStorage.Delete(p.chatID); err != nil {
		return err
	}
	return notificationStorage.DeleteByID(p.notifID)
}

// metaTitle extracts the "title" field from a metadataStorage.Get result - comparing this field
// rather than the raw bytes is required for the sqlite backend, whose Get rebuilds JSON from
// named columns rather than echoing back whatever was passed to Set (see probeMetaTitle doc).
func metaTitle(data []byte) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	title, _ := m["title"].(string)
	return title
}

// probeCounts snapshots the shared kanban/chat probe locations' current sizes. Kanban is
// append-only and a chat file accumulates one row per message, and - unlike the scratch backup
// target each case gets - the probes themselves are seeded into the real, shared live storages,
// so an earlier case (e.g. caseRunProducesFullSet, which never mutates/restores its own probes,
// or caseSelectiveBackup, whose metadata-only restore deliberately never reverts kanban/chat)
// can leave permanent state behind. A case must compare against a baseline captured before its
// own seedProbes call, not a hardcoded absolute count.
type probeCounts struct {
	kanbanEvents int
	chatCount    int
}

func countProbes() (probeCounts, error) {
	events, err := kanbanStorage.GetEvents(probeKanbanFolder, probeKanbanFilePath, nil, nil, 0)
	if err != nil {
		return probeCounts{}, err
	}
	_, chatCount, err := chatStorage.GetPage(probeChatFilePath, 10, 0)
	if err != nil {
		return probeCounts{}, err
	}
	return probeCounts{kanbanEvents: len(events), chatCount: chatCount}, nil
}

// verifyProbes reports whether every probe is back to its original, pre-mutation state. baseline
// must be captured (via countProbes) before this case's own seedProbes call - see probeCounts.
func verifyProbes(p *probes, baseline probeCounts) (bool, string, error) {
	metaVal, err := metadataStorage.Get(probeMetaKey)
	if err != nil {
		return false, "", err
	}
	cacheVal, err := cacheStorage.Get(probeCacheKey)
	if err != nil {
		return false, "", err
	}
	configVal, err := configStorage.Get(probeConfigKey)
	if err != nil {
		return false, "", err
	}
	searchVal, err := searchStorage.GetIndexedContent(probeSearchPath)
	if err != nil {
		return false, "", err
	}
	events, err := kanbanStorage.GetEvents(probeKanbanFolder, probeKanbanFilePath, nil, nil, 0)
	if err != nil {
		return false, "", err
	}
	_, chatCount, err := chatStorage.GetPage(probeChatFilePath, 10, 0)
	if err != nil {
		return false, "", err
	}
	notifs, err := notificationStorage.GetRecent(50)
	if err != nil {
		return false, "", err
	}
	notifFound := false
	for _, n := range notifs {
		if n.ID == p.notifID {
			notifFound = true
			break
		}
	}

	metaOK := metaTitle(metaVal) == probeMetaTitle
	cacheOK := bytes.Equal(cacheVal, probeCacheValue)
	configOK := bytes.Equal(configVal, probeConfigValue)
	searchOK := bytes.Equal(searchVal, probeSearchValue)
	kanbanOK := len(events) == baseline.kanbanEvents+1
	chatOK := chatCount == baseline.chatCount+1

	ok := metaOK && cacheOK && configOK && searchOK && kanbanOK && chatOK && notifFound
	detail := fmt.Sprintf("meta=%v cache=%v config=%v search=%v kanbanEvents=%d(want %d) chatCount=%d(want %d) notifFound=%v",
		metaOK, cacheOK, configOK, searchOK, len(events), baseline.kanbanEvents+1, chatCount, baseline.chatCount+1, notifFound)
	return ok, detail, nil
}

// scratchTarget creates a fresh local backup target in a throwaway temp directory, never the
// real KNOV_BACKUPS_PATH, so nothing this suite creates ever shows up on /system/backup or gets
// mixed into real rotation. Returns a cleanup func the caller must defer.
func scratchTarget() (backup.BackupTarget, func(), error) {
	dir, err := os.MkdirTemp("", "knov-backuptest-target-*")
	if err != nil {
		return nil, nil, err
	}
	target, err := backup.NewLocalTarget(dir)
	if err != nil {
		os.RemoveAll(dir)
		return nil, nil, err
	}
	return target, func() { os.RemoveAll(dir) }, nil
}

// extractTarGz extracts r (a backup.Run-produced archive) into destDir - backup's own
// extraction (untar) is unexported, so cases that need to inspect an archive's contents
// directly (rather than round-tripping it through Restore) get their own minimal copy here.
func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		target := filepath.Join(destDir, filepath.FromSlash(name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("refusing to extract entry outside destination: %s", hdr.Name)
		}
		if hdr.FileInfo().IsDir() || strings.HasSuffix(hdr.Name, "/") {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
}

// reinitStorages re-opens every sqlite-backed storage's db handle after a direct
// backup.Restore() call closed it (see each sqlite storage's Restore doc - normally only a
// real process restart reopens it). Mirrors main.go's own Init sequence exactly, so a case
// calling backup.Restore in-process doesn't leave the live app's storages broken for the rest
// of this run. config is json-backed (no handle to close), so it's not re-initialized here.
func reinitStorages() error {
	cfg := configmanager.GetAppConfig()
	if err := metadataStorage.Init(cfg.MetadataStorageProvider, cfg.StoragePath); err != nil {
		return err
	}
	if err := cacheStorage.Init(cfg.CacheStorageProvider, cfg.StoragePath); err != nil {
		return err
	}
	if err := chatStorage.Init(cfg.StoragePath); err != nil {
		return err
	}
	if err := kanbanStorage.Init(cfg.KanbanEventsEnabled, cfg.KanbanEventsProvider, cfg.StoragePath); err != nil {
		return err
	}
	if err := notificationStorage.Init(cfg.StoragePath); err != nil {
		return err
	}
	if err := searchStorage.Init(cfg.SearchStorageProvider, cfg.StoragePath); err != nil {
		return err
	}
	return nil
}

// restoreAndReinit wraps backup.Restore with the reinitStorages call above - every case in this
// suite that calls backup.Restore directly (bypassing job.RunRestore's os.Exit-then-real-restart)
// must go through this instead of calling backup.Restore on its own. Reinit also runs on
// ErrRestoreIncomplete (not just success), since that error means at least one storage's handle
// may already be closed - skipping reinit there would leave the rest of this run's cases broken.
func restoreAndReinit(target backup.BackupTarget, setName string) error {
	err := backup.Restore(target, setName)
	if err != nil && !errors.Is(err, backup.ErrRestoreIncomplete) {
		return err
	}
	if reinitErr := reinitStorages(); reinitErr != nil {
		return reinitErr
	}
	return err
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
