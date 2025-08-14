// Package server ..
package server

import (
	"github.com/labstack/echo/v4"
)

func setupRoutes(e *echo.Echo) {
	e.GET("/", handleHome)
	e.GET("/home", handleHome)
	e.GET("/help", handleHelp)
	e.GET("/settings", handleSettings)
	e.GET("/search", handleSearch)
	e.GET("/docs", handleDocsRoot)
	e.GET("/docs/:title", handleDocs)
	e.GET("/playground", handlePlayground)
	e.GET("/plugins", handlePlugins)
}
