// Package main ..
package main

import (
	"knov/internal/configmanager"
	"knov/internal/server"
	"knov/internal/thememanager"
)

// @title Knov API
// @version 1.0
// @description KNOV API \n http://localhost:1324
// @host localhost:1324
// @BasePath /
func main() {
	configmanager.InitConfig()

	thememanager.Init()
	server.StartServerChi()
}
