// Package main ..
package main

import (
	"embed"
	"time"

	"knov/internal/cacheStorage"
	"knov/internal/chatStorage"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/fonts"
	"knov/internal/git"
	"knov/internal/job"
	"knov/internal/kanbanStorage"
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/notificationStorage"
	"knov/internal/parser"
	"knov/internal/pdfexport"
	"knov/internal/searchStorage"
	"knov/internal/server"

	"knov/internal/test"
	// every in-app test suite self-registers (test.Register for run-all,
	// job.RegisterSuiteRunner for its own admin button) in its own init() - internal/job
	// doesn't import any of them directly to avoid a cycle for suites that themselves need
	// to import internal/job (e.g. jobstest), so these blank imports are what actually
	// trigger each suite's init() (see internal/job/externalsuite.go).
	_ "knov/internal/test/browsetest"
	_ "knov/internal/test/chattest"
	_ "knov/internal/test/connectionstest"
	_ "knov/internal/test/dashboardtest"
	_ "knov/internal/test/editorstest"
	_ "knov/internal/test/exporttest"
	_ "knov/internal/test/filtertest"
	_ "knov/internal/test/githistorytest"
	_ "knov/internal/test/jobstest"
	_ "knov/internal/test/kanbantest"
	_ "knov/internal/test/logstest"
	_ "knov/internal/test/mediatest"
	_ "knov/internal/test/metadatatest"
	_ "knov/internal/test/notificationstest"
	_ "knov/internal/test/parsertest"
	_ "knov/internal/test/searchtest"
	_ "knov/internal/test/settingstest"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

//go:embed static/*
var staticFS embed.FS

//go:embed themes/builtin
var builtinThemeFS embed.FS

//go:embed docs
var docsFS embed.FS

// @title Knov API
// @version 1.0
// @description KNOV API \n http://localhost:1324
// @host localhost:1324
// @BasePath /
func main() {
	server.SetStaticFiles(staticFS)
	server.SetDocsFiles(docsFS)
	thememanager.SetBuiltinFiles(builtinThemeFS)
	test.SetDocsFiles(docsFS)

	logging.Init()
	logging.InitInterceptor()
	configmanager.InitAppConfig()
	translation.Init()

	if data, err := staticFS.ReadFile("static/font-awesome/ttf/7-3-1/Font Awesome 7 Free-Solid-900.ttf"); err == nil {
		pdfexport.SetIconFont(data)
	} else {
		logging.LogError(logging.KeyPdfExport, "failed to load pdf export icon font: %v", err)
	}
	loadFonts()

	if err := git.EnsureRemote(); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to configure git remote: %v", err)
	}
	git.EnsureRepoConfig()

	// initialize content storage (creates data/docs and data/media directories)
	if err := contentStorage.Init(); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize content storage: %v", err)
		return
	}

	// initialize content handlers
	contentHandler.Init()

	// initialize parsers
	parser.Init()

	// initialize storage backends
	appConfig := configmanager.GetAppConfig()

	if err := configStorage.Init(appConfig.ConfigStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize config storage: %v", err)
		return
	}

	if err := metadataStorage.Init(appConfig.MetadataStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize metadata storage: %v", err)
		return
	}

	if err := kanbanStorage.Init(appConfig.KanbanEventsEnabled, appConfig.KanbanEventsProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize kanban storage: %v", err)
		return
	}

	if err := cacheStorage.Init(appConfig.CacheStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize cache storage: %v", err)
		return
	}

	if err := searchStorage.Init(appConfig.SearchStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize search storage: %v", err)
		return
	}

	if err := chatStorage.Init(appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize chat storage: %v", err)
		return
	}

	if err := notificationStorage.Init(appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize notification storage: %v", err)
		return
	}

	configmanager.InitSettings()
	translation.SetLanguage(configmanager.GetLanguage())

	thememanager.InitThemeManager()
	// register filter index regeneration to run after every metadata rebuild
	files.OnMetadataRebuild = filter.RegenerateAllIndexes

	go func() {
		if err := job.RunSearchReindex(); err != nil {
			logging.LogError(logging.KeyApp, "failed to run startup search index: %v", err)
		}
	}()
	go func() {
		time.Sleep(2 * time.Minute)
		if err := job.RunMetadataRebuild(); err != nil {
			logging.LogError(logging.KeyApp, "failed to run startup metadata rebuild: %v", err)
		}
	}()

	go func() {
		time.Sleep(5 * time.Minute)
		job.Start()
	}()

	server.StartServerChi()
}

// loadFonts registers every embedded font family from the fonts manifest
// with pdfexport (families with no Dir are skipped — those are core fonts
// needing no registration). Families that ship no bold/italic/boldItalic
// pass nil for those styles, and pdfexport falls back to its nearest
// available style at render time.
func loadFonts() {
	read := func(dir, name string) []byte {
		if name == "" {
			return nil
		}
		data, err := staticFS.ReadFile("static/fonts/" + dir + "/" + name)
		if err != nil {
			logging.LogError(logging.KeyPdfExport, "failed to load pdf export font %s/%s: %v", dir, name, err)
			return nil
		}
		return data
	}
	for _, f := range fonts.Families {
		if f.Dir == "" {
			continue
		}
		pdfexport.RegisterFont(f.Name, read(f.Dir, f.Regular), read(f.Dir, f.Bold), read(f.Dir, f.Italic), read(f.Dir, f.BoldItalic))
	}
}
