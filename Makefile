# Variables
APP_NAME := gotest

# ------------- actual usage -------------
dev: swaggo-api-init templ-generate 
	go run ./cmd

prod: swaggo-api-init templ-generate translation
	go build -o $(APP_NAME) ./cmd

rmt: 
	rm ./themes/*.so

# ------------- helper -------------
translation:
	cd internal/translation && go generate

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

swaggo-api-init:
	swag init -g cmd/main.go




.PHONY: templ-generate dev swaggo-api-init rmt translation
