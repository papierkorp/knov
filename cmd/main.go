// Package main ..
package main

import (
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

// @title Knov API
// @version 1.0
// @description KNOV API \n http://localhost:1324
// @host localhost:1324
// @BasePath /
func main() {
	translation.Init()
	translation.SetLanguage(configmanager.GetLanguage())

	configmanager.Init()
	thememanager.Init()
	if err := files.InitSearch(); err != nil {
		logging.LogError("failed to initialize search: %v", err)
	} else {
		files.IndexAllFiles()
	}
	server.StartServerChi()
}
