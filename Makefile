# Variables
APP_NAME := knov

# ------------- actual usage -------------
dev: swaggo-api-init templ-generate
	KNOV_LOG_LEVEL=debug KNOV_DATA_PATH="/home/markus/test" go run ./cmd

prod: swaggo-api-init templ-generate translation
	go build -o bin/$(APP_NAME) ./cmd

# ------------- helper -------------
rmt:
	rm ./themes/*.so

translation:
	cd internal/translation && go generate

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

swaggo-api-init:
	swag init -g cmd/main.go -d . -o internal/server/api

.PHONY: templ-generate dev swaggo-api-init rmt translation prod
