# Variables
APP_NAME := gotest

translation:
	cd internal/translation && go generate

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

swaggo-api-init:
	swag init -g cmd/main.go

dev: swaggo-api-init templ-generate  translation
	go run ./cmd

rmt: 
	rm ./themes/*.so

.PHONY: templ-generate dev swaggo-api-init rmt translation
