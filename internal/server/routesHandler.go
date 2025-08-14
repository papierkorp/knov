// Package server ..
package server

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
	"github.com/papierkorp/knov/internal/thememanager"
)

func _render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response().Writer)
}

func handleHome(c echo.Context) error {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Home()
	if err != nil {
		return err
	}
	return _render(c, component)
}

func handlePlayground(c echo.Context) error {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Playground()
	if err != nil {
		return err
	}
	return _render(c, component)
}

func handleHelp(c echo.Context) error {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Help()
	if err != nil {
		return err
	}
	return _render(c, component)
}

func handleSettings(c echo.Context) error {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Settings()
	if err != nil {
		return err
	}
	return _render(c, component)
}

func handleSearch(c echo.Context) error {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Search()
	if err != nil {
		return err
	}
	return _render(c, component)
}

func handleDocsRoot(c echo.Context) error {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().DocsRoot()
	if err != nil {
		return err
	}
	return _render(c, component)
}

func handleDocs(c echo.Context) error {
	title := c.Param("title")
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Docs(title)
	if err != nil {
		return err
	}
	return _render(c, component)
}

func handlePlugins(c echo.Context) error {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Plugins()
	if err != nil {
		return err
	}
	return _render(c, component)
}
