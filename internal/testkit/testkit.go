// Package testkit boots the real app (same init sequence as main.go)
// against a temp data/storage dir and a fresh git repo, then exposes the
// real chi router via httptest.
//
// All app state (contentStorage, metadataStorage, searchStorage, ...) is
// held in package-level singletons, so only one test app can be "live" at a
// time. Do not call t.Parallel() in tests that use NewApp.
package testkit

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"knov/internal/cacheStorage"
	"knov/internal/chatStorage"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/kanbanStorage"
	"knov/internal/metadataStorage"
	"knov/internal/notificationStorage"
	"knov/internal/parser"
	"knov/internal/searchStorage"
	"knov/internal/server"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

// repoThemesPath resolves the absolute path to the repo's themes/ dir, so
// tests can load real themes from disk regardless of the test's working
// directory.
func repoThemesPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller for repo root lookup")
	}
	// this file lives at internal/testkit/testkit.go
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	themesPath := filepath.Join(repoRoot, "themes")
	if _, err := os.Stat(themesPath); err != nil {
		t.Fatalf("themes dir not found at %s: %v", themesPath, err)
	}
	return themesPath
}

// NewApp initializes the app against a temp data/storage dir (with a
// fresh local git repo) and returns an httptest.Server backed by the real
// router. The server and all temp files are cleaned up automatically.
func NewApp(t *testing.T) *httptest.Server {
	t.Helper()

	dataPath := filepath.Join(t.TempDir(), "data")
	storagePath := filepath.Join(t.TempDir(), "storage")
	logsPath := filepath.Join(t.TempDir(), "logs")

	env := map[string]string{
		"KNOV_DATA_PATH":             dataPath,
		"KNOV_THEMES_PATH":           repoThemesPath(t),
		"KNOV_STORAGE_PATH":          storagePath,
		"KNOV_LOGS_PATH":             logsPath,
		"KNOV_LOG_FILE_ENABLED":      "false",
		"KNOV_GIT_REMOTE":            "",
		"KNOV_GIT_AUTO_PUSH":         "false",
		"KNOV_KANBAN_EVENTS_ENABLED": "false",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	configmanager.InitAppConfig()
	appConfig := configmanager.GetAppConfig()

	if err := contentStorage.Init(); err != nil {
		t.Fatalf("contentStorage.Init: %v", err)
	}
	contentHandler.Init()
	parser.Init()

	if err := configStorage.Init(appConfig.ConfigStorageProvider, appConfig.StoragePath); err != nil {
		t.Fatalf("configStorage.Init: %v", err)
	}
	if err := metadataStorage.Init(appConfig.MetadataStorageProvider, appConfig.StoragePath); err != nil {
		t.Fatalf("metadataStorage.Init: %v", err)
	}
	if err := kanbanStorage.Init(appConfig.KanbanEventsEnabled, appConfig.KanbanEventsProvider, appConfig.StoragePath); err != nil {
		t.Fatalf("kanbanStorage.Init: %v", err)
	}
	if err := cacheStorage.Init(appConfig.CacheStorageProvider, appConfig.StoragePath); err != nil {
		t.Fatalf("cacheStorage.Init: %v", err)
	}
	if err := searchStorage.Init(appConfig.SearchStorageProvider, appConfig.StoragePath); err != nil {
		t.Fatalf("searchStorage.Init: %v", err)
	}
	if err := chatStorage.Init(appConfig.StoragePath); err != nil {
		t.Fatalf("chatStorage.Init: %v", err)
	}
	if err := notificationStorage.Init(appConfig.StoragePath); err != nil {
		t.Fatalf("notificationStorage.Init: %v", err)
	}

	configmanager.InitSettings()
	translation.Init()
	translation.SetLanguage(configmanager.GetLanguage())

	thememanager.InitThemeManager()

	ts := httptest.NewServer(server.NewRouter())
	t.Cleanup(ts.Close)

	return ts
}
