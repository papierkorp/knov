// Package server ...
package server

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func Start() {
	e := echo.New()

	e.Static("/static", "static")
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	setupRoutes(e)

	e.Logger.Fatal(e.Start(":1323"))
}
