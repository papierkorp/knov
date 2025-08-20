// Package main ..
package main

import (
	"knov/internal/server"
	"knov/internal/thememanager"
)

func main() {
	thememanager.Init()
	server.StartServerChi()
}
