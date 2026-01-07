// Package main ..
package main

import (
	"embed"
	"time"

	"knov/internal/configmanager"
	"knov/internal/cronjob"
	"knov/internal/logging"
	"knov/internal/repository"
	"knov/internal/search"
	"knov/internal/server"
	"knov/internal/storage"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

//go:embed static/*
var staticFS embed.FS

//go:embed themes/builtin
var builtinThemeFS embed.FS

// @title Knov API
// @version 1.0
// @description KNOV API \n http://localhost:1324
// @host localhost:1324
// @BasePath /
func main() {
	server.SetStaticFiles(staticFS)
	thememanager.SetBuiltinFiles(builtinThemeFS)

	configmanager.InitAppConfig()
	translation.Init()
	storage.InitStorages(
		configmanager.GetConfigStorageProvider(),
		configmanager.GetMetadataStorageProvider(),
		configmanager.GetCacheStorageProvider(),
		configmanager.GetStoragePath(),
	)
	repository.InitRepositories()
	configmanager.InitUserSettings()
	translation.SetLanguage(configmanager.GetLanguage())

	thememanager.InitThemeManager()
	if err := search.InitSearch(); err != nil {
		logging.LogError("failed to initialize search: %v", err)
	}

	go func() {
		time.Sleep(2 * time.Minute)
		cronjob.Start()
		defer cronjob.Stop()
	}()

	server.StartServerChi()
}
