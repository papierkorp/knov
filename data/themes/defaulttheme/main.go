// Package defaulttheme ..
package main

import (
	"knov/data/themes/defaulttheme/templates"

	"github.com/a-h/templ"
)

// DefaultTheme ..
type DefaultTheme struct{}

// Theme ..
var Theme DefaultTheme

// Home ...
func (t *DefaultTheme) Home() (templ.Component, error) {
	return templates.Home(), nil
}

func main() {}
