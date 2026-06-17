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
	"knov/internal/cronjob"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/kanbanStorage"
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/notificationStorage"
	"knov/internal/parser"
	"knov/internal/search"
	"knov/internal/searchStorage"
	"knov/internal/server"

	"knov/internal/testdata"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

//go:embed static/*
var staticFS embed.FS

//go:embed themes/builtin
var builtinThemeFS embed.FS

//go:embed themes/rail
var railThemeFS embed.FS

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
	thememanager.SetRailFiles(railThemeFS)
	testdata.SetDocsFiles(docsFS)

	logging.Init()
	logging.InitInterceptor()
	configmanager.InitAppConfig()
	translation.Init()

	if err := git.EnsureRemote(); err != nil {
		logging.LogWarning("failed to configure git remote: %v", err)
	}

	// initialize content storage (creates data/docs and data/media directories)
	if err := contentStorage.Init(); err != nil {
		logging.LogError("failed to initialize content storage: %v", err)
		return
	}

	// initialize content handlers
	contentHandler.Init()

	// initialize parsers
	parser.Init()

	// initialize storage backends
	appConfig := configmanager.GetAppConfig()

	if err := configStorage.Init(appConfig.ConfigStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError("failed to initialize config storage: %v", err)
		return
	}

	if err := metadataStorage.Init(appConfig.MetadataStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError("failed to initialize metadata storage: %v", err)
		return
	}

	if err := kanbanStorage.Init(appConfig.KanbanEventsEnabled, appConfig.KanbanEventsProvider, appConfig.StoragePath); err != nil {
		logging.LogError("failed to initialize kanban storage: %v", err)
		return
	}

	if err := cacheStorage.Init(appConfig.CacheStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError("failed to initialize cache storage: %v", err)
		return
	}

	if err := searchStorage.Init(appConfig.SearchStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError("failed to initialize search storage: %v", err)
		return
	}

	if err := chatStorage.Init(appConfig.StoragePath); err != nil {
		logging.LogError("failed to initialize chat storage: %v", err)
		return
	}

	if err := notificationStorage.Init(appConfig.StoragePath); err != nil {
		logging.LogError("failed to initialize notification storage: %v", err)
		return
	}

	configmanager.InitUserSettings()
	translation.SetLanguage(configmanager.GetLanguage())

	thememanager.InitThemeManager()
	// register filter index regeneration to run after every metadata rebuild
	files.OnMetadataRebuild = filter.RegenerateAllIndexes

	go func() {
		if err := search.InitSearch(); err != nil {
			logging.LogError("failed to initialize search: %v", err)
		}
	}()
	go func() {
		time.Sleep(2 * time.Minute)
		if err := files.MetaDataLinksRebuild(); err != nil {
			logging.LogError("failed to run startup metadata rebuild: %v", err)
		}
	}()

	go func() {
		time.Sleep(5 * time.Minute)
		cronjob.Start()
		defer cronjob.Stop()
	}()

	server.StartServerChi()
}
