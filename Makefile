# Variables
APP_NAME := gotest

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

dev: templ-generate
	go run ./cmd

.PHONY: templ-generate dev
