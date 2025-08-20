// Package defaulttheme ..
package main

import (
	"knov/data/themes/dark/templates"

	"github.com/a-h/templ"
)

// Dark ..
type Dark struct{}

// Theme ..
var Theme Dark

// Home ...
func (t *Dark) Home() (templ.Component, error) {
	return templates.Home(), nil
}

func main() {}
