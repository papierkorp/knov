# Variables
APP_NAME := gotest

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

swaggo-api-init:
	swag init -g cmd/main.go

dev: swaggo-api-init templ-generate 
	go run ./cmd

.PHONY: templ-generate dev swaggo-api-init
